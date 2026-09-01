import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/svelte";
import type { SmartAccountsSyncStatus } from "$lib/api";

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
    createSmartAccountsBrowserPairing: vi.fn(),
    getSmartAccountsBrowserPairing: vi.fn(),
    createSmartAccountsBrowserDiscovery: vi.fn(),
    submitSmartAccountsBrowserDiscoveryReceipt: vi.fn(),
    getSmartAccountsBrowserDiscoveryReceipt: vi.fn(),
    reviewSmartAccountsBrowserCSVSchema: vi.fn(),
    createSmartAccountsBrowserCapture: vi.fn(),
    resumeSmartAccountsBrowserCapture: vi.fn(),
    getSmartAccountsBrowserCaptureStatus: vi.fn(),
    startSmartAccountsBrowserCaptureWorkflow: vi.fn(),
    getSmartAccountsBrowserCaptureWorkflowStatus: vi.fn(),
    configureSmartAccountsSync: vi.fn(),
    getSmartAccountsSyncStatus: vi.fn(),
    requestSmartAccountsSyncDryRun: vi.fn(),
    previewSmartAccountsPackage: vi.fn(),
    applySmartAccountsPackage: vi.fn(),
    resolveSmartAccountsTolerancePolicy: vi.fn(),
    getSmartAccountsPackageArchiveCoverage: vi.fn(),
    previewSmartAccountsReferenceMasters: vi.fn(),
    applySmartAccountsReferenceMasters: vi.fn(),
    issueSmartAccountsBrowserMasterDetails: vi.fn(),
    getSmartAccountsBrowserMasterDetailStatus: vi.fn(),
    resumeSmartAccountsBrowserMasterDetail: vi.fn(),
    startSmartAccountsBrowserOnboarding: vi.fn(),
    getSmartAccountsBrowserOnboarding: vi.fn(),
    issueSmartAccountsBrowserOnboardingCatalog: vi.fn(),
    getSmartAccountsBrowserOnboardingCatalog: vi.fn(),
    startSmartAccountsBrowserOnboardingBatch: vi.fn(),
    getSmartAccountsBrowserOnboardingBatch: vi.fn(),
    resumeSmartAccountsBrowserOnboardingBatch: vi.fn(),
    prepareSmartAccountsBrowserOnboardingBatchWorkflow: vi.fn(),
    getSmartAccountsBrowserOnboardingBatchWorkflow: vi.fn(),
    resumeSmartAccountsBrowserOnboardingBatchWorkflow: vi.fn(),
    acquireSmartAccountsBrowserOnboardingBatchDiscovery: vi.fn(),
    reissueSmartAccountsBrowserOnboardingBatchDiscovery: vi.fn(),
    completeSmartAccountsBrowserOnboardingBatchDiscovery: vi.fn(),
    requireSmartAccountsBrowserOnboardingBatchSchema: vi.fn(),
    refreshSmartAccountsBrowserOnboardingBatchSchema: vi.fn(),
    confirmSmartAccountsBrowserOnboardingBatchSchema: vi.fn(),
    openSmartAccountsBrowserOnboardingBatchTransfer: vi.fn(),
    confirmSmartAccountsBrowserOnboardingBatchTransfer: vi.fn(),
    acquireSmartAccountsBrowserOnboardingBatchCapture: vi.fn(),
    completeSmartAccountsBrowserOnboardingBatchCapture: vi.fn(),
    previewSmartAccountsBrowserOnboardingBatchSource: vi.fn(),
    getSmartAccountsReconciliation: vi.fn(),
    getSmartAccountsTenantReconciliation: vi.fn(),
    getSmartAccountsReconciliationRollup: vi.fn(),
    evaluateSmartAccountsReconciliation: vi.fn(),
    getMyTenants: vi.fn(),
    createTenant: vi.fn(),
  },
}));

vi.mock("$lib/api", async () => {
  const actual = await vi.importActual<typeof import("$lib/api")>("$lib/api");
  return { ...actual, api: apiMock };
});

import SmartAccountsSyncControl from "$lib/components/SmartAccountsSyncControl.svelte";

type RelayReadinessReply = {
  relay_protocol_version?: string;
  capture_manifest_version?: string;
  workflow_plan_version?: string;
  smartaccounts_session_state?: "signed_in" | "signed_out" | "unknown";
  relay_build_version?: string;
};

let readinessPings: Array<Record<string, unknown>> = [];
let automaticRelayReadinessReply:
  ((nonce: string) => RelayReadinessReply) | null = null;

function relayReadinessReply(
  nonce: string,
  overrides: RelayReadinessReply = {},
): RelayReadinessReply & {
  source: string;
  type: string;
  version: number;
  nonce: string;
} {
  return {
    source: "smartaccounts-browser-relay",
    type: "smartaccounts-browser-relay.readiness.v1",
    version: 1,
    nonce,
    relay_protocol_version: "smartaccounts-browser-relay-v1",
    capture_manifest_version: "smartaccounts-brave-ui-v2",
    workflow_plan_version: "smartaccounts-browser-capture-plan-v1",
    smartaccounts_session_state: "signed_in" as const,
    ...overrides,
  };
}

function syncStatus(
  overrides: Partial<SmartAccountsSyncStatus> = {},
): SmartAccountsSyncStatus {
  return {
    provider: "smartaccounts",
    source_company_id: "sa-company-hmb-9881",
    source_company_name: "Hold My Beer OÜ",
    configured: false,
    secret_reference_configured: false,
    smartaccounts_gl_authoritative: true,
    invoice_payment_mode: "NON_POSTING",
    capture_status: "NOT_REQUESTED",
    plan_status: "NOT_REQUESTED",
    reconciliation_status: "PENDING_SOURCE_EVIDENCE",
    financial_apply_eligible: false,
    explicit_confirmation_required: true,
    financial_writes_started: false,
    next_action:
      "Connect and validate SmartAccounts through the private bridge.",
    ...overrides,
  };
}

async function handoffVisibleCompanyCatalog() {
  await fireEvent.click(
    screen.getByRole("button", { name: "Choose companies manually" }),
  );
  await waitFor(() =>
    expect(
      apiMock.issueSmartAccountsBrowserOnboardingCatalog,
    ).toHaveBeenCalledTimes(1),
  );
  window.dispatchEvent(
    new MessageEvent("message", {
      source: window,
      origin: window.location.origin,
      data: {
        source: "smartaccounts-browser-relay",
        type: "smartaccounts-browser-relay.source-catalog-result.v1",
        version: 1,
        catalog_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        workflow_id: "c436c224-5df5-4b4d-a772-1897f9147400",
        nonce: "N".repeat(43),
        status: "accepted",
        catalog_count: 2,
        catalog_sha256: "c".repeat(64),
      },
    }),
  );
  await screen.findByText("Create or reuse isolated company tenants");
}

describe("SmartAccountsSyncControl", () => {
  beforeEach(() => {
    vi.clearAllMocks();
    readinessPings = [];
    automaticRelayReadinessReply = (nonce) => relayReadinessReply(nonce);
    const originalPostMessage = window.postMessage.bind(window);
    vi.spyOn(window, "postMessage").mockImplementation(
      (message, targetOrigin) => {
        originalPostMessage(message, targetOrigin);
        if (!message || typeof message !== "object") return;
        const ping = message as Record<string, unknown>;
        if (
          ping.source !== "open-accounting" ||
          ping.type !==
            "open-accounting.smartaccounts-browser-readiness-ping.v1" ||
          typeof ping.nonce !== "string"
        )
          return;
        readinessPings.push(ping);
        const reply = automaticRelayReadinessReply?.(ping.nonce);
        if (!reply) return;
        window.dispatchEvent(
          new MessageEvent("message", {
            source: window,
            origin: window.location.origin,
            data: relayReadinessReply(ping.nonce, reply),
          }),
        );
      },
    );
    apiMock.configureSmartAccountsSync.mockResolvedValue(
      syncStatus({
        configured: true,
        secret_reference_configured: true,
        source_company_name: "Hold My Beer OÜ",
      }),
    );
    apiMock.requestSmartAccountsSyncDryRun.mockResolvedValue(
      syncStatus({
        configured: true,
        secret_reference_configured: true,
        capture_status: "AWAITING_BRIDGE_CAPTURE",
        plan_status: "AWAITING_CAPTURE",
      }),
    );
    apiMock.previewSmartAccountsPackage.mockResolvedValue({
      id: "preview-1",
      package_id: "package-1",
      status: "PREVIEW_READY",
      preview_sha256: "a".repeat(64),
      financial_writes_planned: true,
      financial_writes_applied: false,
      journals: [{}],
      account_imports: [{}],
      non_posting_record_count: 5,
    });
    apiMock.resolveSmartAccountsTolerancePolicy.mockResolvedValue({
      policy_id: "policy-1",
      algorithm_version: "smartaccounts-exact-match-v1",
      label: "Exact match — zero variance",
      tolerance_policy_sha256: "server-derived-digest-never-rendered",
      approved_at: "2026-08-28T10:00:00Z",
    });
    apiMock.applySmartAccountsPackage.mockResolvedValue({
      id: "preview-1",
      package_id: "package-1",
      status: "APPLIED",
      preview_sha256: "a".repeat(64),
      financial_writes_planned: true,
      financial_writes_applied: true,
      journals: [{}],
      account_imports: [{}],
      non_posting_record_count: 5,
    });
    apiMock.previewSmartAccountsReferenceMasters.mockResolvedValue({
      id: "reference-preview-1",
      tenant_id: "tenant-1",
      package_id: "package-1",
      source_company_id: "sa-browser-v1-123456",
      status: "PREVIEW_READY",
      preview_sha256: "b".repeat(64),
      actions: [
        {
          entity_type: "account",
          external_id: "a-1000",
          target_id: "11111111-1111-4111-8111-111111111111",
          revision: "c".repeat(64),
          action: "CREATE",
        },
      ],
      reconciliation: [
        {
          entity_type: "account",
          source_records: 1,
          create_planned: 1,
          already_applied: 0,
          review_required: 0,
          tombstones: 0,
        },
      ],
    });
    apiMock.applySmartAccountsReferenceMasters.mockResolvedValue({
      id: "reference-preview-1",
      tenant_id: "tenant-1",
      package_id: "package-1",
      source_company_id: "sa-browser-v1-123456",
      status: "APPLIED",
      preview_sha256: "b".repeat(64),
      actions: [],
      reconciliation: [
        {
          entity_type: "account",
          source_records: 1,
          create_planned: 1,
          already_applied: 0,
          review_required: 0,
          tombstones: 0,
        },
      ],
    });
    const masterIssue = (
      resource_id: "clients" | "vendors" | "articles",
      sequence: 1 | 2 | 3,
      run_id: string,
    ) => ({
      run_id,
      tenant_id: "tenant-1",
      source_company_id: "sa-browser-v1-123456",
      manifest_version: "smartaccounts-browser-master-detail-v1",
      resource_id,
      schema_id: `${resource_id}_detail_v1`,
      source_schema: `smartaccounts-browser-master-detail-v1/${resource_id}_detail_v1`,
      contract: {
        version: "smartaccounts-browser-master-detail-v1",
        resource: resource_id,
        origin: "https://sa.smartaccounts.eu",
        list_page_path: `/et/${resource_id}`,
        detail_path_prefix: `/et/${resource_id}/`,
        detail_result_page_path: `/et/${resource_id}`,
        fields: [],
      },
      contract_sha256: "a".repeat(64),
      approval_sha256: "b".repeat(64),
      scope: {
        from_inclusive: "2026-08-28",
        to_inclusive: "2026-08-28",
        cutoff_at: "2026-08-28T10:00:00Z",
      },
      snapshot_policy: "current_snapshot_only",
      snapshot_date: "2026-08-28",
      expires_at: "2026-08-28T10:10:00Z",
      transfer_consent: {
        version: 1,
        confirmed: true,
        confirmed_at: "2026-08-28T10:00:00Z",
      },
      capture_token: `${resource_id}-capability-not-rendered`,
      sequence,
    });
    apiMock.issueSmartAccountsBrowserMasterDetails.mockResolvedValue({
      batch_id: "master-batch-1",
      issues: [
        masterIssue("clients", 1, "master-client-run"),
        masterIssue("vendors", 2, "master-vendor-run"),
        masterIssue("articles", 3, "master-article-run"),
      ],
    });
    apiMock.getSmartAccountsBrowserMasterDetailStatus.mockImplementation(
      async (_tenantID: string, runID: string) => {
        const resource_id = runID.includes("vendor")
          ? "vendors"
          : runID.includes("article")
            ? "articles"
            : "clients";
        return {
          ...masterIssue(
            resource_id,
            resource_id === "clients" ? 1 : resource_id === "vendors" ? 2 : 3,
            runID,
          ),
          status: "open",
          capture_token: undefined,
        };
      },
    );
    apiMock.resumeSmartAccountsBrowserMasterDetail.mockResolvedValue(
      masterIssue("clients", 1, "master-client-run"),
    );
    apiMock.createSmartAccountsBrowserPairing.mockResolvedValue({
      pairing_id: "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
      pairing_token: "relay-token-not-rendered",
      expires_at: "2026-08-27T15:10:00Z",
    });
    apiMock.getSmartAccountsBrowserPairing.mockResolvedValue({
      pairing_id: "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
      status: "CLAIMED",
      expires_at: "2026-08-27T15:10:00Z",
      source_company_id: "sa-browser-v1-123456",
    });
    const discoveryResources = [
      "account_turnover",
      "annual_report",
      "articles",
      "balance_sheet",
      "bank_payments",
      "cash_flow_statement",
      "cash_payments",
      "client_invoices",
      "client_offers",
      "client_orders",
      "clients",
      "depreciations",
      "fixed_asset_depreciation_report",
      "fixed_assets",
      "general_ledger",
      "income_statement",
      "journal_entries",
      "other_reports",
      "salaries",
      "tsd_returns",
      "vat_returns",
      "vendor_invoices",
      "vendor_orders",
      "vendors",
      "wage_reports",
      "warehouse_inventory",
      "warehouse_movements",
      "warehouse_movements_report",
      "warehouses",
      "worker_absences",
      "workers",
    ];
    apiMock.createSmartAccountsBrowserDiscovery.mockResolvedValue({
      discovery_id: "417f6fec-1994-4cfe-8ea6-bb7281d3050f",
      tenant_id: "tenant-1",
      source_company_id: "sa-browser-v1-123456",
      manifest_version: "smartaccounts-brave-ui-v2",
      resource_ids: discoveryResources,
      expires_at: "2026-08-28T10:10:00Z",
      discovery_consent: {
        version: 1,
        confirmed: true,
        confirmed_at: "2026-08-28T10:00:00Z",
        scope: "metadata_only",
      },
    });
    apiMock.submitSmartAccountsBrowserDiscoveryReceipt.mockResolvedValue({
      discovery_id: "417f6fec-1994-4cfe-8ea6-bb7281d3050f",
      status: "completed",
      manifest_version: "smartaccounts-brave-ui-v2",
      contract_version: "smartaccounts-brave-discovery-contract-v1",
      contract_sha256: "d".repeat(64),
      resource_count: 31,
      capture_ready_count: 1,
      filter_contract_required_count: 23,
      page_only_contract_required_count: 7,
      private_endpoint_required_count: 0,
      binding_blocked_count: 0,
    });
    apiMock.getSmartAccountsBrowserDiscoveryReceipt.mockResolvedValue({
      discovery_id: "417f6fec-1994-4cfe-8ea6-bb7281d3050f",
      status: "completed",
      manifest_version: "smartaccounts-brave-ui-v2",
      contract_version: "smartaccounts-brave-discovery-contract-v1",
      contract_sha256: "d".repeat(64),
      resource_count: 31,
      capture_ready_count: 1,
      filter_contract_required_count: 23,
      page_only_contract_required_count: 7,
      private_endpoint_required_count: 0,
      binding_blocked_count: 0,
    });
    apiMock.reviewSmartAccountsBrowserCSVSchema.mockResolvedValue({
      resource_id: "general_ledger",
      schema_id: "general_ledger_csv_v1",
      status: "registered",
      approval_sha256: "e".repeat(64),
    });
    const generalLedgerScope = {
      mode: "partial",
      from_inclusive: "2024-01-01",
      to_inclusive: "2026-08-28",
      cutoff_at: "2026-08-28T10:00:00Z",
      resource_ids: ["general_ledger"],
    };
    const capture = {
      run_id: "10bb2ae9-6c95-4ece-92a9-3c6c11bfc5b2",
      tenant_id: "tenant-1",
      capture_token: "capture-token-not-rendered",
      expires_at: "2026-08-28T10:10:00Z",
      source_company_id: "sa-browser-v1-123456",
      manifest_version: "smartaccounts-brave-ui-v2",
      scope: generalLedgerScope,
      status: "open",
      transfer_consent: {
        version: 1,
        confirmed: true,
        confirmed_at: "2026-08-28T10:00:00Z",
      },
    };
    const plan = {
      version: "smartaccounts-browser-capture-plan-v1",
      from_date_policy: "OWNER_EXPLICIT_FROM_DATE",
      run_id: capture.run_id,
      tenant_id: "tenant-1",
      source_company_id: "sa-browser-v1-123456",
      manifest_version: "smartaccounts-brave-ui-v2",
      scope: generalLedgerScope,
      eligible_resource_ids: ["general_ledger"],
    };
    apiMock.startSmartAccountsBrowserCaptureWorkflow.mockResolvedValue({
      workflow_id: "89c27eff-4dce-4820-acfb-f30d52b85af3",
      status: "CAPTURE_ISSUED",
      plan,
      capture,
    });
    apiMock.resumeSmartAccountsBrowserCapture.mockResolvedValue({
      ...capture,
      capture_token: "resumed-capture-token-not-rendered",
      expires_at: "2026-08-28T10:20:00Z",
      transfer_consent: {
        version: 1,
        confirmed: true,
        confirmed_at: "2026-08-28T10:10:00Z",
      },
    });
    apiMock.getSmartAccountsBrowserCaptureStatus.mockResolvedValue({
      run_id: capture.run_id,
      tenant_id: "tenant-1",
      source_company_id: "sa-browser-v1-123456",
      manifest_version: "smartaccounts-brave-ui-v2",
      status: "open",
      scope: generalLedgerScope,
      resources: [
        {
          resource_id: "general_ledger",
          coverage: "export_csv",
          status: "pending",
        },
      ],
    });
    apiMock.getSmartAccountsBrowserCaptureWorkflowStatus.mockResolvedValue({
      workflow_id: "89c27eff-4dce-4820-acfb-f30d52b85af3",
      status: "CAPTURE_ISSUED",
      plan,
      progress: {
        run_id: capture.run_id,
        tenant_id: "tenant-1",
        source_company_id: "sa-browser-v1-123456",
        manifest_version: "smartaccounts-brave-ui-v2",
        status: "open",
        scope: generalLedgerScope,
        resources: [
          {
            resource_id: "general_ledger",
            coverage: "export_csv",
            status: "pending",
          },
        ],
      },
    });
    apiMock.getMyTenants.mockResolvedValue([]);
    apiMock.createTenant.mockResolvedValue({
      id: "created-tenant-1",
      name: "Company B",
      slug: "company-b-9876",
    });
    apiMock.startSmartAccountsBrowserOnboarding.mockResolvedValue({
      bindings: [],
    });
    apiMock.issueSmartAccountsBrowserOnboardingCatalog.mockResolvedValue({
      catalog_id: "b436c224-5df5-4b4d-a772-1897f9147400",
      workflow_id: "c436c224-5df5-4b4d-a772-1897f9147400",
      catalog_token: "catalog-token-not-rendered-012345678901234567",
      nonce: "N".repeat(43),
      issued_at: "2026-08-28T10:00:00Z",
      expires_at: "2026-08-28T10:02:00Z",
      catalog_digest_intent: {
        version: "smartaccounts-browser-source-catalog-intent-v1",
        catalog_schema_version: "smartaccounts-browser-source-catalog-v1",
        source_id_version: "sa-browser-v1",
        digest_algorithm: "sha256",
      },
      catalog_consent: {
        version: 1,
        confirmed: true,
        confirmed_at: "2026-08-28T10:00:00Z",
        scope: "visible_company_catalog",
      },
    });
    apiMock.getSmartAccountsBrowserOnboardingCatalog.mockResolvedValue({
      catalog_id: "b436c224-5df5-4b4d-a772-1897f9147400",
      workflow_id: "c436c224-5df5-4b4d-a772-1897f9147400",
      status: "ACCEPTED",
      catalog_sha256: "c".repeat(64),
      catalog_count: 2,
      observed_at: "2026-08-28T10:00:01Z",
      expires_at: "2026-08-28T10:10:01Z",
      companies: [
        { source_company_id: "sa-browser-v1-1234", display_name: "Company A" },
        { source_company_id: "sa-browser-v1-9876", display_name: "Company B" },
      ],
    });
    apiMock.startSmartAccountsBrowserOnboardingBatch.mockResolvedValue({
      batch: {
        batch_id: "d436c224-5df5-4b4d-a772-1897f9147400",
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        relay_observed_at: "2026-08-28T10:00:01Z",
        mode: "all",
        selected_sources: [
          {
            source_company_id: "sa-browser-v1-1234",
            source_company_name: "Company A",
          },
          {
            source_company_id: "sa-browser-v1-9876",
            source_company_name: "Company B",
          },
        ],
        observed_source_ids: ["sa-browser-v1-1234", "sa-browser-v1-9876"],
        observed_sources_sha256: "c".repeat(64),
        manifest_sha256: "d".repeat(64),
        status: "PENDING",
        created_at: "2026-08-28T10:00:02Z",
        updated_at: "2026-08-28T10:00:02Z",
      },
      outcomes: [],
      pairing_issues: [],
      reused: false,
    });
    apiMock.getSmartAccountsReconciliation.mockRejectedValue(
      new Error("Request failed with status 404"),
    );
    apiMock.getSmartAccountsReconciliationRollup.mockResolvedValue({
      batch_id: "d436c224-5df5-4b4d-a772-1897f9147400",
      status: "IN_PROGRESS",
      selected_count: 0,
      pass_count: 0,
      pending_count: 0,
      review_count: 0,
      failure_count: 0,
    });
  });

  afterEach(() => {
    cleanup();
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
    window.sessionStorage.clear();
  });

  it("gates Brave actions on a nonce-bound zero-data relay readiness response", async () => {
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });

    await screen.findByText("Relay ready — SmartAccounts is signed in.");
    expect(readinessPings).toHaveLength(1);
    expect(readinessPings[0]).toMatchObject({
      source: "open-accounting",
      type: "open-accounting.smartaccounts-browser-readiness-ping.v1",
      version: 1,
    });
    expect(readinessPings[0].nonce).toMatch(/^[A-Za-z0-9_-]{43}$/);
    expect(readinessPings[0].issued_at).toEqual(expect.any(String));
    expect(readinessPings[0].expires_at).toEqual(expect.any(String));
    expect(Object.keys(readinessPings[0]).sort()).toEqual([
      "expires_at",
      "issued_at",
      "nonce",
      "source",
      "type",
      "version",
    ]);
    expect(
      screen.getByRole("button", { name: "Connect with Brave (no API key)" }),
    ).toBeEnabled();
    expect(
      screen.getByRole("button", { name: "Start all-company safe sync" }),
    ).toBeEnabled();
    expect(
      Object.keys(window.sessionStorage).filter((key) =>
        key.includes("readiness"),
      ),
    ).toEqual([]);
  });

  it("shows an exact safe relay build version while accepting legacy versionless readiness", async () => {
    automaticRelayReadinessReply = (nonce) =>
      relayReadinessReply(nonce, { relay_build_version: "0.2.7" });
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    expect(await screen.findByText("Installed relay build: 0.2.7")).toBeInTheDocument();
    expect(
      screen.getByText("Relay ready — SmartAccounts is signed in."),
    ).toBeInTheDocument();
  });

  it("reissues an expired master-detail capability only after renewed per-run consent", async () => {
    window.sessionStorage.setItem(
      "open-accounting:smartaccounts-source:tenant-1",
      "sa-browser-v1-123456",
    );
    apiMock.getSmartAccountsSyncStatus.mockResolvedValue(
      syncStatus({
        source_company_id: "sa-browser-v1-123456",
        configured: true,
        secret_reference_configured: true,
      }),
    );
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await screen.findByText("Relay ready — SmartAccounts is signed in.");
    await fireEvent.click(
      screen.getByLabelText(/I authorize this tenant’s paired source/),
    );
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Authorize current master snapshots",
      }),
    );
    const resume = await screen.findByRole("button", {
      name: "Resume clients snapshot",
    });
    expect(resume).toBeDisabled();
    await fireEvent.click(
      screen.getAllByLabelText(
        /I reconfirm this exact tenant, source, resource/,
      )[0],
    );
    await fireEvent.click(resume);
    await waitFor(() =>
      expect(
        apiMock.resumeSmartAccountsBrowserMasterDetail,
      ).toHaveBeenCalledWith("tenant-1", "master-client-run", {
        transfer_consent_confirmed: true,
      }),
    );
    expect(
      screen.queryByText(/capability-not-rendered/),
    ).not.toBeInTheDocument();
  });

  it("rejects a data-bearing readiness reply without rendering the unexpected value", async () => {
    automaticRelayReadinessReply = null;
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await waitFor(() => expect(readinessPings).toHaveLength(1));
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: {
          ...relayReadinessReply(String(readinessPings[0].nonce)),
          unexpected_source_row: "never-rendered-source-row",
        },
      }),
    );

    await screen.findByText(/Relay needs reload or update/);
    expect(
      screen.queryByText("never-rendered-source-row"),
    ).not.toBeInTheDocument();
  });

  it("accepts a browser catalog only once when catalog, workflow, nonce, digest, and count all match", async () => {
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await fireEvent.click(
      screen.getByRole("button", { name: "Choose companies manually" }),
    );
    await waitFor(() =>
      expect(
        apiMock.issueSmartAccountsBrowserOnboardingCatalog,
      ).toHaveBeenCalledTimes(1),
    );
    // The reload checkpoint contains only the opaque receipt ID, never the
    // one-time relay capability or the picker contents.
    expect(
      window.sessionStorage.getItem(
        "open-accounting:smartaccounts-browser-onboarding-catalog-receipt:v1",
      ),
    ).toBe("b436c224-5df5-4b4d-a772-1897f9147400");
    const base = {
      source: "smartaccounts-browser-relay",
      type: "smartaccounts-browser-relay.source-catalog-result.v1",
      version: 1,
      catalog_id: "b436c224-5df5-4b4d-a772-1897f9147400",
      workflow_id: "c436c224-5df5-4b4d-a772-1897f9147400",
      nonce: "N".repeat(43),
      status: "accepted",
      catalog_count: 2,
      catalog_sha256: "c".repeat(64),
    };
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: { ...base, workflow_id: "wrong-workflow" },
      }),
    );
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: { ...base, unexpected_source_row: "never-rendered" },
      }),
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(
      apiMock.getSmartAccountsBrowserOnboardingCatalog,
    ).not.toHaveBeenCalled();
    expect(screen.queryByText("never-rendered")).not.toBeInTheDocument();
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: base,
      }),
    );
    await screen.findByText("Create or reuse isolated company tenants");
    expect(
      apiMock.getSmartAccountsBrowserOnboardingCatalog,
    ).toHaveBeenCalledTimes(1);
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: base,
      }),
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(
      apiMock.getSmartAccountsBrowserOnboardingCatalog,
    ).toHaveBeenCalledTimes(1);
  });

  it("uses one owner click to hand off the catalog and create the immutable all-company pairing batch", async () => {
    render(SmartAccountsSyncControl, { tenantId: "" });
    await screen.findByText("Relay ready — SmartAccounts is signed in.");
    await fireEvent.click(
      screen.getByRole("button", { name: "Start all-company safe sync" }),
    );
    await waitFor(() =>
      expect(
        apiMock.issueSmartAccountsBrowserOnboardingCatalog,
      ).toHaveBeenCalledWith({
        catalog_consent: expect.objectContaining({
          confirmed: true,
          scope: "visible_company_catalog",
        }),
      }),
    );
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: {
          source: "smartaccounts-browser-relay",
          type: "smartaccounts-browser-relay.source-catalog-result.v1",
          version: 1,
          catalog_id: "b436c224-5df5-4b4d-a772-1897f9147400",
          workflow_id: "c436c224-5df5-4b4d-a772-1897f9147400",
          nonce: "N".repeat(43),
          status: "accepted",
          catalog_count: 2,
          catalog_sha256: "c".repeat(64),
        },
      }),
    );
    await waitFor(() =>
      expect(
        apiMock.startSmartAccountsBrowserOnboardingBatch,
      ).toHaveBeenCalledWith({
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        mode: "all",
        selected_source_ids: ["sa-browser-v1-1234", "sa-browser-v1-9876"],
        owner_confirmed: true,
      }),
    );
    expect(
      screen.queryByRole("heading", {
        name: "Create or reuse isolated company tenants",
      }),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("catalog-token-not-rendered-012345678901234567"),
    ).not.toBeInTheDocument();
  });

  it("releases a nonresponsive catalog action without retaining a relay capability", async () => {
    vi.useFakeTimers();
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await fireEvent.click(
      screen.getByRole("button", { name: "Choose companies manually" }),
    );
    await vi.advanceTimersByTimeAsync(5_000);
    expect(
      screen.getByText(/did not return the visible-company catalog/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Start all-company safe sync" }),
    ).toBeEnabled();
    expect(
      screen.queryByText("catalog-token-not-rendered-012345678901234567"),
    ).not.toBeInTheDocument();
  });

  it("releases an awaiting-browser catalog response so the one-click action can be retried", async () => {
    render(SmartAccountsSyncControl, { tenantId: "" });
    await fireEvent.click(
      screen.getByRole("button", { name: "Choose companies manually" }),
    );
    await waitFor(() =>
      expect(
        apiMock.issueSmartAccountsBrowserOnboardingCatalog,
      ).toHaveBeenCalledTimes(1),
    );
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: {
          source: "smartaccounts-browser-relay",
          type: "smartaccounts-browser-relay.source-catalog-result.v1",
          version: 1,
          catalog_id: "b436c224-5df5-4b4d-a772-1897f9147400",
          workflow_id: "c436c224-5df5-4b4d-a772-1897f9147400",
          nonce: "N".repeat(43),
          status: "awaiting_browser",
        },
      }),
    );
    await screen.findByText(
      "The relay is waiting for the visible SmartAccounts company picker.",
    );
    expect(
      screen.getByRole("button", { name: "Start all-company safe sync" }),
    ).toBeEnabled();
  });

  it("renders only a fixed catalog-failure stage and rejects data-bearing catalog results", async () => {
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await fireEvent.click(
      screen.getByRole("button", { name: "Choose companies manually" }),
    );
    await waitFor(() =>
      expect(
        apiMock.issueSmartAccountsBrowserOnboardingCatalog,
      ).toHaveBeenCalledTimes(1),
    );
    const base = {
      source: "smartaccounts-browser-relay",
      type: "smartaccounts-browser-relay.source-catalog-result.v1",
      version: 1,
      catalog_id: "b436c224-5df5-4b4d-a772-1897f9147400",
      workflow_id: "c436c224-5df5-4b4d-a772-1897f9147400",
      nonce: "N".repeat(43),
      status: "catalog_blocked",
    };
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: {
          ...base,
          failure_stage: "picker_unstable",
          source_html: "never-rendered-source-html",
        },
      }),
    );
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(
      screen.queryByText(/did not settle to one complete list/),
    ).not.toBeInTheDocument();
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: { ...base, failure_stage: "picker_unstable" },
      }),
    );
    await screen.findByText(/did not settle to one complete list/);
    expect(
      screen.queryByText("never-rendered-source-html"),
    ).not.toBeInTheDocument();
  });

  it("does not let an older accepted catalog GET overwrite a newer issued catalog", async () => {
    let resolveFirst: ((value: unknown) => void) | undefined;
    let resolveSecond: ((value: unknown) => void) | undefined;
    apiMock.issueSmartAccountsBrowserOnboardingCatalog
      .mockResolvedValueOnce({
        catalog_id: "11111111-1111-4111-8111-111111111111",
        workflow_id: "21111111-1111-4111-8111-111111111111",
        catalog_token: "A".repeat(43),
        nonce: "B".repeat(43),
        issued_at: "2026-08-28T10:00:00Z",
        expires_at: "2026-08-28T10:02:00Z",
        catalog_digest_intent: {
          version: "smartaccounts-browser-source-catalog-intent-v1",
          catalog_schema_version: "smartaccounts-browser-source-catalog-v1",
          source_id_version: "sa-browser-v1",
          digest_algorithm: "sha256",
        },
        catalog_consent: {
          version: 1,
          confirmed: true,
          confirmed_at: "2026-08-28T10:00:00Z",
          scope: "visible_company_catalog",
        },
      })
      .mockResolvedValueOnce({
        catalog_id: "31111111-1111-4111-8111-111111111111",
        workflow_id: "41111111-1111-4111-8111-111111111111",
        catalog_token: "C".repeat(43),
        nonce: "D".repeat(43),
        issued_at: "2026-08-28T10:00:01Z",
        expires_at: "2026-08-28T10:02:01Z",
        catalog_digest_intent: {
          version: "smartaccounts-browser-source-catalog-intent-v1",
          catalog_schema_version: "smartaccounts-browser-source-catalog-v1",
          source_id_version: "sa-browser-v1",
          digest_algorithm: "sha256",
        },
        catalog_consent: {
          version: 1,
          confirmed: true,
          confirmed_at: "2026-08-28T10:00:01Z",
          scope: "visible_company_catalog",
        },
      });
    apiMock.getSmartAccountsBrowserOnboardingCatalog
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveFirst = resolve;
          }),
      )
      .mockImplementationOnce(
        () =>
          new Promise((resolve) => {
            resolveSecond = resolve;
          }),
      );
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    const readCatalog = screen.getByRole("button", {
      name: "Choose companies manually",
    });
    await fireEvent.click(readCatalog);
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: {
          source: "smartaccounts-browser-relay",
          type: "smartaccounts-browser-relay.source-catalog-result.v1",
          version: 1,
          catalog_id: "11111111-1111-4111-8111-111111111111",
          workflow_id: "21111111-1111-4111-8111-111111111111",
          nonce: "B".repeat(43),
          status: "accepted",
          catalog_count: 1,
          catalog_sha256: "1".repeat(64),
        },
      }),
    );
    await waitFor(() =>
      expect(
        apiMock.getSmartAccountsBrowserOnboardingCatalog,
      ).toHaveBeenCalledTimes(1),
    );
    await fireEvent.click(readCatalog);
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: {
          source: "smartaccounts-browser-relay",
          type: "smartaccounts-browser-relay.source-catalog-result.v1",
          version: 1,
          catalog_id: "31111111-1111-4111-8111-111111111111",
          workflow_id: "41111111-1111-4111-8111-111111111111",
          nonce: "D".repeat(43),
          status: "accepted",
          catalog_count: 1,
          catalog_sha256: "2".repeat(64),
        },
      }),
    );
    await waitFor(() =>
      expect(
        apiMock.getSmartAccountsBrowserOnboardingCatalog,
      ).toHaveBeenCalledTimes(2),
    );
    resolveFirst?.({
      catalog_id: "11111111-1111-4111-8111-111111111111",
      workflow_id: "21111111-1111-4111-8111-111111111111",
      status: "ACCEPTED",
      catalog_sha256: "1".repeat(64),
      catalog_count: 1,
      observed_at: "2026-08-28T10:00:01Z",
      expires_at: "2026-08-28T10:10:01Z",
      companies: [
        { source_company_id: "sa-browser-v1-1", display_name: "Old Company" },
      ],
    });
    await new Promise((resolve) => setTimeout(resolve, 0));
    expect(screen.queryByText("Old Company")).not.toBeInTheDocument();
    resolveSecond?.({
      catalog_id: "31111111-1111-4111-8111-111111111111",
      workflow_id: "41111111-1111-4111-8111-111111111111",
      status: "ACCEPTED",
      catalog_sha256: "2".repeat(64),
      catalog_count: 1,
      observed_at: "2026-08-28T10:00:02Z",
      expires_at: "2026-08-28T10:10:02Z",
      companies: [
        { source_company_id: "sa-browser-v1-2", display_name: "New Company" },
      ],
    });
    await screen.findByLabelText("All 1 relay-observed companies");
  });

  it("restores a current accepted catalog receipt through the owner status endpoint after a page reload", async () => {
    const catalogID = "b436c224-5df5-4b4d-a772-1897f9147400";
    window.sessionStorage.setItem(
      "open-accounting:smartaccounts-browser-onboarding-catalog-receipt:v1",
      catalogID,
    );

    render(SmartAccountsSyncControl, { tenantId: "" });

    await waitFor(() =>
      expect(
        apiMock.getSmartAccountsBrowserOnboardingCatalog,
      ).toHaveBeenCalledWith(catalogID),
    );
    expect(
      await screen.findByRole("heading", {
        name: "Create or reuse isolated company tenants",
      }),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        /Restored the current accepted metadata-only company catalog/,
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByLabelText("All 2 relay-observed companies"),
    ).toBeInTheDocument();
    expect(
      window.sessionStorage.getItem(
        "open-accounting:smartaccounts-browser-onboarding-catalog-receipt:v1",
      ),
    ).toBe(catalogID);
    expect(
      screen.queryByText("catalog-token-not-rendered-012345678901234567"),
    ).not.toBeInTheDocument();
  });

  it("clears an unavailable catalog checkpoint instead of leaving the page reading indefinitely", async () => {
    const catalogID = "b436c224-5df5-4b4d-a772-1897f9147400";
    apiMock.getSmartAccountsBrowserOnboardingCatalog.mockRejectedValueOnce(
      new Error("not found"),
    );
    window.sessionStorage.setItem(
      "open-accounting:smartaccounts-browser-onboarding-catalog-receipt:v1",
      catalogID,
    );

    render(SmartAccountsSyncControl, { tenantId: "" });

    await screen.findByText(
      /current accepted company catalog receipt is no longer available/,
    );
    expect(
      window.sessionStorage.getItem(
        "open-accounting:smartaccounts-browser-onboarding-catalog-receipt:v1",
      ),
    ).toBeNull();
    expect(
      screen.queryByRole("heading", {
        name: "Create or reuse isolated company tenants",
      }),
    ).not.toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Start all-company safe sync" }),
    ).toBeEnabled();
  });

  it("ignores a readiness reply bound to another nonce before accepting the current response", async () => {
    automaticRelayReadinessReply = null;
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await waitFor(() => expect(readinessPings).toHaveLength(1));
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: relayReadinessReply("A".repeat(43)),
      }),
    );

    expect(
      screen.getByText("Checking the installed Brave relay…"),
    ).toBeInTheDocument();
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: relayReadinessReply(String(readinessPings[0].nonce)),
      }),
    );
    await screen.findByText("Relay ready — SmartAccounts is signed in.");
  });

  it("rejects a stale relay protocol response and leaves Brave actions disabled", async () => {
    automaticRelayReadinessReply = (nonce) =>
      relayReadinessReply(nonce, {
        workflow_plan_version: "smartaccounts-browser-capture-plan-v0",
      });
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });

    await screen.findByText(/Relay needs reload or update/);
    expect(
      screen.getByRole("button", { name: "Connect with Brave (no API key)" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Start all-company safe sync" }),
    ).toBeDisabled();
  });

  it("shows a sign-in action and leaves Brave actions disabled when the relay is signed out", async () => {
    automaticRelayReadinessReply = (nonce) =>
      relayReadinessReply(nonce, { smartaccounts_session_state: "signed_out" });
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });

    await screen.findByText(/SmartAccounts is signed out/);
    expect(
      screen.getByRole("button", { name: "Connect with Brave (no API key)" }),
    ).toBeDisabled();
    expect(
      screen.getByRole("button", { name: "Start all-company safe sync" }),
    ).toBeDisabled();
  });

  it("shows a missing relay action when no same-window readiness response arrives", async () => {
    vi.useFakeTimers();
    automaticRelayReadinessReply = null;
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });

    await vi.advanceTimersByTimeAsync(3_000);
    expect(screen.getByText(/Relay was not detected/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Connect with Brave (no API key)" }),
    ).toBeDisabled();
  });

  it("retries with a fresh in-memory nonce and enables Brave actions only after signed-in readiness", async () => {
    let replies = 0;
    automaticRelayReadinessReply = (nonce) =>
      relayReadinessReply(nonce, {
        smartaccounts_session_state:
          ++replies === 1 ? "signed_out" : "signed_in",
      });
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });

    await screen.findByText(/SmartAccounts is signed out/);
    await fireEvent.click(
      screen.getByRole("button", { name: "Check Browser Relay again" }),
    );
    await screen.findByText("Relay ready — SmartAccounts is signed in.");
    expect(readinessPings).toHaveLength(2);
    expect(readinessPings[1].nonce).not.toBe(readinessPings[0].nonce);
    expect(
      screen.getByRole("button", { name: "Connect with Brave (no API key)" }),
    ).toBeEnabled();
  });

  it("restores only the opaque source binding after a page refresh and reloads safe progress", async () => {
    window.sessionStorage.setItem(
      "open-accounting:smartaccounts-source:tenant-1",
      "sa-company-hmb-9881",
    );
    apiMock.getSmartAccountsSyncStatus.mockResolvedValue(
      syncStatus({ configured: true, secret_reference_configured: true }),
    );

    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });

    await waitFor(() =>
      expect(apiMock.getSmartAccountsSyncStatus).toHaveBeenCalledWith(
        "tenant-1",
        "sa-company-hmb-9881",
      ),
    );
    expect(screen.getByText("Hold My Beer OÜ")).toBeInTheDocument();
  });

  it("uses the signed-in Brave relay by default without rendering or persisting its one-time token", async () => {
    apiMock.getSmartAccountsSyncStatus.mockResolvedValue(
      syncStatus({
        configured: true,
        secret_reference_configured: true,
        source_company_id: "sa-browser-v1-123456",
        source_company_name: "SmartAccounts browser session",
        capture_status: "AWAITING_BRAVE_BROWSER_CAPTURE",
        plan_status: "AWAITING_CAPTURE",
      }),
    );
    const pairingEvents: Array<Record<string, unknown>> = [];
    const observePairing = (event: MessageEvent<Record<string, unknown>>) => {
      if (
        event.data?.type ===
        "open-accounting.smartaccounts-browser-pairing-issued.v1"
      )
        pairingEvents.push(event.data);
    };
    window.addEventListener("message", observePairing);
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });

    await fireEvent.click(
      screen.getByRole("button", { name: "Connect with Brave (no API key)" }),
    );
    await waitFor(() =>
      expect(apiMock.createSmartAccountsBrowserPairing).toHaveBeenCalledWith(
        "tenant-1",
      ),
    );
    await waitFor(() => expect(pairingEvents).toHaveLength(1));
    expect(pairingEvents[0]).toMatchObject({
      source: "open-accounting",
      version: 1,
      pairing_id: "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
    });
    expect(
      screen.queryByText("relay-token-not-rendered"),
    ).not.toBeInTheDocument();
    expect(
      window.sessionStorage.getItem(
        "open-accounting:smartaccounts-source:tenant-1",
      ),
    ).toBeNull();

    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: {
          source: "smartaccounts-browser-relay",
          type: "smartaccounts-browser-relay.pairing-result.v1",
          pairing_id: "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
          status: "claimed",
        },
      }),
    );
    await waitFor(() =>
      expect(apiMock.getSmartAccountsBrowserPairing).toHaveBeenCalledWith(
        "tenant-1",
        "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
      ),
    );
    await waitFor(() =>
      expect(apiMock.getSmartAccountsSyncStatus).toHaveBeenCalledWith(
        "tenant-1",
        "sa-browser-v1-123456",
      ),
    );
    expect(
      window.sessionStorage.getItem(
        "open-accounting:smartaccounts-source:tenant-1",
      ),
    ).toBe("sa-browser-v1-123456");
    window.removeEventListener("message", observePairing);
  });

  it("issues a full server-derived discovery manifest with separate header consent and renders only the safe receipt", async () => {
    apiMock.getSmartAccountsSyncStatus.mockResolvedValue(
      syncStatus({
        configured: true,
        secret_reference_configured: true,
        source_company_id: "sa-browser-v1-123456",
        source_company_name: "SmartAccounts browser session",
      }),
    );
    window.sessionStorage.setItem(
      "open-accounting:smartaccounts-source:tenant-1",
      "sa-browser-v1-123456",
    );
    apiMock.createSmartAccountsBrowserDiscovery.mockResolvedValueOnce({
      discovery_id: "417f6fec-1994-4cfe-8ea6-bb7281d3050f",
      tenant_id: "tenant-1",
      source_company_id: "sa-browser-v1-123456",
      manifest_version: "smartaccounts-brave-ui-v2",
      resource_ids: [
        "account_turnover",
        "annual_report",
        "articles",
        "balance_sheet",
        "bank_payments",
        "cash_flow_statement",
        "cash_payments",
        "client_invoices",
        "client_offers",
        "client_orders",
        "clients",
        "depreciations",
        "fixed_asset_depreciation_report",
        "fixed_assets",
        "general_ledger",
        "income_statement",
        "journal_entries",
        "other_reports",
        "salaries",
        "tsd_returns",
        "vat_returns",
        "vendor_invoices",
        "vendor_orders",
        "vendors",
        "wage_reports",
        "warehouse_inventory",
        "warehouse_movements",
        "warehouse_movements_report",
        "warehouses",
        "worker_absences",
        "workers",
      ],
      expires_at: "2026-08-28T10:10:00Z",
      discovery_consent: {
        version: 1,
        confirmed: true,
        confirmed_at: "2026-08-28T10:00:00Z",
        scope: "metadata_and_header_probe",
        response_header_probe_confirmed: true,
      },
    });
    const discoveryEvents: Array<Record<string, unknown>> = [];
    const observe = (event: MessageEvent<Record<string, unknown>>) => {
      if (
        event.data?.type ===
        "open-accounting.smartaccounts-browser-discovery-issued.v1"
      )
        discoveryEvents.push(event.data);
    };
    window.addEventListener("message", observe);
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });

    await screen.findByRole("heading", {
      name: "Discover browser contract coverage",
    });
    const start = screen.getByRole("button", {
      name: "Discover browser contracts",
    });
    expect(start).toBeDisabled();
    await fireEvent.click(
      screen.getByLabelText(/I approve this metadata-only browser discovery/),
    );
    await fireEvent.click(
      screen.getByLabelText(
        /I separately approve bounded CSV header-name probing/,
      ),
    );
    await fireEvent.click(start);
    await waitFor(() => expect(discoveryEvents).toHaveLength(1));
    expect(discoveryEvents[0]).toMatchObject({
      source: "open-accounting",
      type: "open-accounting.smartaccounts-browser-discovery-issued.v1",
      version: 1,
      discovery_id: "417f6fec-1994-4cfe-8ea6-bb7281d3050f",
      source_company_id: "sa-browser-v1-123456",
      manifest_version: "smartaccounts-brave-ui-v2",
      discovery_consent: {
        scope: "metadata_and_header_probe",
        response_header_probe_confirmed: true,
      },
    });
    expect(discoveryEvents[0].resource_ids as string[]).toHaveLength(31);
    expect(discoveryEvents[0].resource_ids).toContain("annual_report");
    expect(discoveryEvents[0].resource_ids).toContain(
      "warehouse_movements_report",
    );
    expect(discoveryEvents[0].resource_ids).not.toEqual(["journal_entries"]);
    expect(apiMock.createSmartAccountsBrowserDiscovery).toHaveBeenCalledWith(
      "tenant-1",
      {
        source_company_id: "sa-browser-v1-123456",
        metadata_only_consent_confirmed: true,
        response_header_probe_confirmed: true,
      },
    );

    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: {
          source: "smartaccounts-browser-relay",
          type: "smartaccounts-browser-relay.discovery-result.v1",
          version: 1,
          discovery_id: "417f6fec-1994-4cfe-8ea6-bb7281d3050f",
          manifest_version: "smartaccounts-brave-ui-v2",
          contract_version: "smartaccounts-brave-discovery-contract-v1",
          status: "completed",
          resources: [
            {
              resource_id: "journal_entries",
              unexpected_source_row: "never-rendered",
            },
          ],
        },
      }),
    );
    await waitFor(() =>
      expect(
        apiMock.submitSmartAccountsBrowserDiscoveryReceipt,
      ).toHaveBeenCalledTimes(1),
    );
    expect(
      apiMock.submitSmartAccountsBrowserDiscoveryReceipt,
    ).toHaveBeenCalledWith(
      "tenant-1",
      "417f6fec-1994-4cfe-8ea6-bb7281d3050f",
      expect.objectContaining({
        type: "smartaccounts-browser-relay.discovery-result.v1",
      }),
    );
    await screen.findByRole("heading", { name: "Redacted discovery receipt" });
    expect(screen.getByText(/31\/31 surfaces recorded/)).toBeInTheDocument();
    expect(screen.queryByText("never-rendered")).not.toBeInTheDocument();
    expect(screen.queryByText("filterResults")).not.toBeInTheDocument();
    const review = screen.getByRole("button", {
      name: "Register reviewed General Ledger CSV schema",
    });
    expect(review).toBeDisabled();
    await fireEvent.click(
      screen.getByLabelText(/I confirm the reviewed General Ledger CSV schema/),
    );
    await fireEvent.click(review);
    await waitFor(() =>
      expect(apiMock.reviewSmartAccountsBrowserCSVSchema).toHaveBeenCalledWith(
        "tenant-1",
        "417f6fec-1994-4cfe-8ea6-bb7281d3050f",
        "general_ledger",
        "general_ledger_csv_v1",
      ),
    );
    expect(
      screen.getByText("registered", { selector: "strong" }),
    ).toBeInTheDocument();
    expect(screen.getByText(/approval digest: e{64}/)).toBeInTheDocument();
    expect(screen.queryByText("sa-browser-v1-123456")).not.toBeInTheDocument();
    await fireEvent.click(
      screen.getByRole("button", { name: "Refresh safe discovery status" }),
    );
    await waitFor(() =>
      expect(
        apiMock.getSmartAccountsBrowserDiscoveryReceipt,
      ).toHaveBeenCalledWith(
        "tenant-1",
        "417f6fec-1994-4cfe-8ea6-bb7281d3050f",
      ),
    );
    window.removeEventListener("message", observe);
  });

  it("hands a server-derived General Ledger workflow token directly to the Brave relay without retaining it in the UI", async () => {
    apiMock.getSmartAccountsSyncStatus.mockResolvedValue(
      syncStatus({
        configured: true,
        secret_reference_configured: true,
        source_company_id: "sa-browser-v1-123456",
        source_company_name: "SmartAccounts browser session",
        capture_status: "AWAITING_BRAVE_BROWSER_CAPTURE",
      }),
    );
    const events: Array<Record<string, unknown>> = [];
    const observe = (event: MessageEvent<Record<string, unknown>>) => {
      if (
        event.data?.type ===
        "open-accounting.smartaccounts-browser-workflow-issued.v1"
      )
        events.push(event.data);
    };
    window.addEventListener("message", observe);
    window.sessionStorage.setItem(
      "open-accounting:smartaccounts-source:tenant-1",
      "sa-browser-v1-123456",
    );
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await waitFor(() =>
      expect(
        screen.getByRole("button", {
          name: "Start General Ledger CSV capture",
        }),
      ).toBeInTheDocument(),
    );
    await fireEvent.input(screen.getByLabelText("History starts"), {
      target: { value: "2024-01-01" },
    });
    await fireEvent.click(
      screen.getByLabelText(
        /I confirm this partial General Ledger CSV transfer/,
      ),
    );
    await fireEvent.click(
      screen.getByRole("button", { name: "Start General Ledger CSV capture" }),
    );
    await waitFor(() => expect(events).toHaveLength(1));
    expect(events[0]).toMatchObject({
      tenant_id: "tenant-1",
      source_company_id: "sa-browser-v1-123456",
      transfer_consent: { version: 1, confirmed: true },
      workflow: {
        version: "smartaccounts-browser-workflow-v1",
        operation: "capture",
        plan: {
          eligible_resource_ids: ["general_ledger"],
          scope: { resource_ids: ["general_ledger"] },
        },
      },
    });
    expect(
      apiMock.startSmartAccountsBrowserCaptureWorkflow,
    ).toHaveBeenCalledWith("tenant-1", {
      source_company_id: "sa-browser-v1-123456",
      from_inclusive: "2024-01-01",
      transfer_consent_confirmed: true,
    });
    const resume = screen.getByRole("button", {
      name: "Resume same Brave capture",
    });
    expect(resume).toBeDisabled();
    await fireEvent.click(
      screen.getByLabelText(
        "I confirm transfer for the same tenant, source, run, and scope again.",
      ),
    );
    await fireEvent.click(resume);
    await waitFor(() =>
      expect(apiMock.resumeSmartAccountsBrowserCapture).toHaveBeenCalledWith(
        "tenant-1",
        "10bb2ae9-6c95-4ece-92a9-3c6c11bfc5b2",
        { transfer_consent_confirmed: true },
      ),
    );
    await waitFor(() => expect(events).toHaveLength(2));
    expect(events[1]).toMatchObject({
      run_id: "10bb2ae9-6c95-4ece-92a9-3c6c11bfc5b2",
      tenant_id: "tenant-1",
      type: "open-accounting.smartaccounts-browser-workflow-issued.v1",
    });
    expect(
      screen.queryByText("capture-token-not-rendered"),
    ).not.toBeInTheDocument();
    expect(
      screen.queryByText("resumed-capture-token-not-rendered"),
    ).not.toBeInTheDocument();
    window.removeEventListener("message", observe);
  });

  it("restores only a safe workflow ID after restart and rotates the same run after renewed consent", async () => {
    apiMock.getSmartAccountsSyncStatus.mockResolvedValue(
      syncStatus({
        configured: true,
        secret_reference_configured: true,
        source_company_id: "sa-browser-v1-123456",
        source_company_name: "SmartAccounts browser session",
      }),
    );
    window.sessionStorage.setItem(
      "open-accounting:smartaccounts-source:tenant-1",
      "sa-browser-v1-123456",
    );
    window.sessionStorage.setItem(
      "open-accounting:smartaccounts-browser-workflow:tenant-1:sa-browser-v1-123456",
      "89c27eff-4dce-4820-acfb-f30d52b85af3",
    );
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await waitFor(() =>
      expect(
        apiMock.getSmartAccountsBrowserCaptureWorkflowStatus,
      ).toHaveBeenCalledWith(
        "tenant-1",
        "89c27eff-4dce-4820-acfb-f30d52b85af3",
      ),
    );
    const resume = await screen.findByRole("button", {
      name: "Resume same Brave capture",
    });
    expect(resume).toBeDisabled();
    await fireEvent.click(
      screen.getByLabelText(
        "I confirm transfer for the same tenant, source, run, and scope again.",
      ),
    );
    await fireEvent.click(resume);
    await waitFor(() =>
      expect(apiMock.resumeSmartAccountsBrowserCapture).toHaveBeenCalledWith(
        "tenant-1",
        "10bb2ae9-6c95-4ece-92a9-3c6c11bfc5b2",
        { transfer_consent_confirmed: true },
      ),
    );
    expect(
      screen.queryByText("capture-token-not-rendered"),
    ).not.toBeInTheDocument();
  });

  it("reads owner-safe staged partial status and keeps GL apply separate until the server resolves an approved accountant policy", async () => {
    apiMock.getSmartAccountsSyncStatus.mockResolvedValue(
      syncStatus({
        configured: true,
        secret_reference_configured: true,
        source_company_id: "sa-browser-v1-123456",
        source_company_name: "SmartAccounts browser session",
        capture_status: "AWAITING_BRAVE_BROWSER_CAPTURE",
      }),
    );
    apiMock.getSmartAccountsBrowserCaptureWorkflowStatus.mockResolvedValue({
      workflow_id: "89c27eff-4dce-4820-acfb-f30d52b85af3",
      status: "CAPTURE_ISSUED",
      plan: {
        version: "smartaccounts-browser-capture-plan-v1",
        from_date_policy: "OWNER_EXPLICIT_FROM_DATE",
        run_id: "10bb2ae9-6c95-4ece-92a9-3c6c11bfc5b2",
        tenant_id: "tenant-1",
        source_company_id: "sa-browser-v1-123456",
        manifest_version: "smartaccounts-brave-ui-v2",
        scope: {
          mode: "partial",
          from_inclusive: "2024-01-01",
          to_inclusive: "2026-08-28",
          cutoff_at: "2026-08-28T10:00:00Z",
          resource_ids: ["general_ledger"],
        },
        eligible_resource_ids: ["general_ledger"],
      },
      progress: {
        run_id: "10bb2ae9-6c95-4ece-92a9-3c6c11bfc5b2",
        tenant_id: "tenant-1",
        source_company_id: "sa-browser-v1-123456",
        manifest_version: "smartaccounts-brave-ui-v2",
        status: "finalized_partial",
        scope: {
          mode: "partial",
          from_inclusive: "2024-01-01",
          to_inclusive: "2026-08-28",
          cutoff_at: "2026-08-28T10:00:00Z",
          resource_ids: ["general_ledger"],
        },
        resources: [
          {
            resource_id: "general_ledger",
            coverage: "export_csv",
            status: "completed",
          },
        ],
        receipt: {
          status: "partial_coverage_recorded",
          ready: false,
          completed_export_count: 1,
          required_export_count: 1,
          blocked_page_only_count: 0,
          finalized_at: "2026-08-28T10:01:00Z",
        },
        staging: {
          package_id: "package-1",
          package_sha256: "a".repeat(64),
          status: "staged_review_required",
          record_chunks_acknowledged: 1,
          artifact_chunks_acknowledged: 1,
          finalized: true,
        },
      },
    });
    window.sessionStorage.setItem(
      "open-accounting:smartaccounts-source:tenant-1",
      "sa-browser-v1-123456",
    );
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await screen.findByRole("button", {
      name: "Start General Ledger CSV capture",
    });
    await fireEvent.input(screen.getByLabelText("History starts"), {
      target: { value: "2024-01-01" },
    });
    await fireEvent.click(
      screen.getByLabelText(
        /I confirm this partial General Ledger CSV transfer/,
      ),
    );
    await fireEvent.click(
      screen.getByRole("button", { name: "Start General Ledger CSV capture" }),
    );
    await screen.findByText(/Partial browser package package-1 is staged/);
    expect(
      screen.getByText(/Scope: partial browser export/),
    ).toBeInTheDocument();
    await waitFor(() =>
      expect(apiMock.previewSmartAccountsPackage).toHaveBeenCalledTimes(1),
    );
    expect(apiMock.previewSmartAccountsPackage).toHaveBeenCalledWith(
      "tenant-1",
      "package-1",
      { use_source_chart: true },
    );
    await fireEvent.click(
      screen.getByRole("button", { name: "Refresh safe capture status" }),
    );
    await fireEvent.click(
      screen.getByRole("button", { name: "Refresh safe capture status" }),
    );
    await waitFor(() =>
      expect(
        apiMock.getSmartAccountsBrowserCaptureWorkflowStatus,
      ).toHaveBeenCalledTimes(3),
    );
    expect(apiMock.previewSmartAccountsPackage).toHaveBeenCalledTimes(1);
    const confirmation = screen.getByLabelText(
      "I reviewed this partial, GL-authoritative plan and want to create and post these journals once.",
    );
    await fireEvent.click(confirmation);
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Confirm and apply reviewed GL plan",
      }),
    );
    await waitFor(() =>
      expect(apiMock.resolveSmartAccountsTolerancePolicy).toHaveBeenCalledWith(
        "tenant-1",
        "sa-browser-v1-123456",
        { package_id: "package-1", preview_id: "preview-1" },
      ),
    );
    await waitFor(() =>
      expect(apiMock.applySmartAccountsPackage).toHaveBeenCalledWith(
        "tenant-1",
        {
          confirm: true,
          preview_id: "preview-1",
          preview_sha256: "a".repeat(64),
          tolerance_policy_id: "policy-1",
        },
      ),
    );
    expect(
      screen.queryByText("server-derived-digest-never-rendered"),
    ).not.toBeInTheDocument();
    expect(
      [...Array(sessionStorage.length)]
        .map(
          (_, index) =>
            sessionStorage.getItem(sessionStorage.key(index) ?? "") ?? "",
        )
        .join("\n"),
    ).not.toContain("server-derived-digest-never-rendered");
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Prepare non-financial master preview",
      }),
    );
    await waitFor(() =>
      expect(apiMock.previewSmartAccountsReferenceMasters).toHaveBeenCalledWith(
        "tenant-1",
        "package-1",
        {},
      ),
    );
    expect(
      screen.getByText(
        /This preview never posts a journal, invoice, or payment/,
      ),
    ).toBeInTheDocument();
    await fireEvent.click(
      screen.getByLabelText(
        /I reviewed this non-financial account, contact, and item master plan/,
      ),
    );
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Confirm and apply reference masters",
      }),
    );
    await waitFor(() =>
      expect(apiMock.applySmartAccountsReferenceMasters).toHaveBeenCalledWith(
        "tenant-1",
        {
          confirm: true,
          preview_id: "reference-preview-1",
          preview_sha256: "b".repeat(64),
        },
      ),
    );
    expect(
      screen.queryByText("capture-token-not-rendered"),
    ).not.toBeInTheDocument();
  });

  it("falls back from the legacy picker whitespace bug to the same bounded metadata catalog and one-click batch", async () => {
    const expiresAt = new Date(Date.now() + 120_000).toISOString();
    apiMock.issueSmartAccountsBrowserOnboardingCatalog.mockResolvedValue({
      catalog_id: "b436c224-5df5-4b4d-a772-1897f9147400",
      workflow_id: "c436c224-5df5-4b4d-a772-1897f9147400",
      catalog_token: "catalog-token-not-rendered-012345678901234567",
      nonce: "N".repeat(43),
      issued_at: new Date().toISOString(),
      expires_at: expiresAt,
      catalog_digest_intent: {
        version: "smartaccounts-browser-source-catalog-intent-v1",
        catalog_schema_version: "smartaccounts-browser-source-catalog-v1",
        source_id_version: "sa-browser-v1",
        digest_algorithm: "sha256",
      },
      catalog_consent: {
        version: 1,
        confirmed: true,
        confirmed_at: new Date().toISOString(),
        scope: "visible_company_catalog",
      },
    });
    let acceptedDigest = "";
    const fetchMock = vi.fn(
      async (_input: RequestInfo | URL, init?: RequestInit) => {
        const handoff = JSON.parse(String(init?.body ?? "{}")) as {
          catalog_sha256: string;
          catalog_count: number;
        };
        acceptedDigest = handoff.catalog_sha256;
        return {
          ok: true,
          json: async () => ({
            status: "accepted",
            catalog_id: "b436c224-5df5-4b4d-a772-1897f9147400",
            catalog_count: handoff.catalog_count,
            catalog_sha256: handoff.catalog_sha256,
          }),
        } as Response;
      },
    );
    vi.stubGlobal("fetch", fetchMock);
    apiMock.getSmartAccountsBrowserOnboardingCatalog.mockImplementation(
      async () => ({
        catalog_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        workflow_id: "c436c224-5df5-4b4d-a772-1897f9147400",
        status: "ACCEPTED",
        catalog_sha256: acceptedDigest,
        catalog_count: 2,
        observed_at: new Date().toISOString(),
        expires_at: new Date(Date.now() + 600_000).toISOString(),
        companies: [
          {
            source_company_id: "sa-browser-v1-1234",
            display_name: "Company A",
          },
          {
            source_company_id: "sa-browser-v1-9876",
            display_name: "Company B",
          },
        ],
      }),
    );

    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await fireEvent.click(
      await screen.findByRole("button", {
        name: "Start all-company safe sync",
      }),
    );
    await waitFor(() =>
      expect(
        apiMock.issueSmartAccountsBrowserOnboardingCatalog,
      ).toHaveBeenCalledTimes(1),
    );
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: {
          source: "smartaccounts-browser-relay",
          type: "smartaccounts-browser-relay.source-catalog-result.v1",
          version: 1,
          catalog_id: "b436c224-5df5-4b4d-a772-1897f9147400",
          workflow_id: "c436c224-5df5-4b4d-a772-1897f9147400",
          nonce: "N".repeat(43),
          status: "catalog_blocked",
          failure_stage: "picker_unstable",
        },
      }),
    );
    await waitFor(() =>
      expect(window.postMessage).toHaveBeenCalledWith(
        {
          source: "open-accounting",
          type: "open-accounting.smartaccounts-browser-source-discovery-request.v1",
          version: 1,
        },
        window.location.origin,
      ),
    );
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: {
          source: "smartaccounts-browser-relay",
          type: "smartaccounts-browser-relay.source-discovery-result.v1",
          version: 1,
          status: "ready",
          companies: [
            {
              sourceCompanyID: "sa-browser-v1-9876",
              sourceCompanyName: "Company B",
            },
            {
              sourceCompanyID: "sa-browser-v1-1234",
              sourceCompanyName: "Company A",
            },
          ],
        },
      }),
    );
    await waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    expect(acceptedDigest).toBe(
      "0e5c2dcf6454d4bc6bfc228e9e43e0d3fb45e0ef26386319e2805ef6c1ef60dc",
    );
    const [, handoffInit] = fetchMock.mock.calls[0];
    expect(handoffInit?.credentials).toBe("omit");
    expect(handoffInit?.headers).toMatchObject({
      Authorization: "Bearer catalog-token-not-rendered-012345678901234567",
      "Content-Type": "application/json",
    });
    await waitFor(() =>
      expect(
        apiMock.startSmartAccountsBrowserOnboardingBatch,
      ).toHaveBeenCalledWith({
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        mode: "all",
        selected_source_ids: ["sa-browser-v1-1234", "sa-browser-v1-9876"],
        owner_confirmed: true,
      }),
    );
    expect(apiMock.createSmartAccountsBrowserCapture).not.toHaveBeenCalled();
    expect(apiMock.applySmartAccountsPackage).not.toHaveBeenCalled();
  });

  it("completes an awaiting relay claim through the bounded metadata fallback without auto-capture", async () => {
    apiMock.startSmartAccountsBrowserOnboardingBatch.mockResolvedValue({
      batch: {
        batch_id: "d436c224-5df5-4b4d-a772-1897f9147400",
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        relay_observed_at: "2026-08-28T10:00:01Z",
        mode: "all",
        selected_sources: [
          {
            source_company_id: "sa-browser-v1-1234",
            source_company_name: "Company A",
          },
          {
            source_company_id: "sa-browser-v1-9876",
            source_company_name: "Company B",
          },
        ],
        observed_source_ids: ["sa-browser-v1-1234", "sa-browser-v1-9876"],
        observed_sources_sha256: "c".repeat(64),
        manifest_sha256: "d".repeat(64),
        status: "PENDING",
        created_at: "2026-08-28T10:00:02Z",
        updated_at: "2026-08-28T10:00:02Z",
      },
      outcomes: [
        {
          source_company_id: "sa-browser-v1-1234",
          source_company_name: "Company A",
          tenant_id: "existing-tenant-1",
          tenant_name: "Company A",
          pairing_id: "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
          status: "PAIRING_ISSUED",
          tenant_created: false,
          tenant_reused: true,
        },
        {
          source_company_id: "sa-browser-v1-9876",
          source_company_name: "Company B",
          tenant_id: "created-tenant-2",
          tenant_name: "Company B",
          pairing_id: "1a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
          status: "PAIRING_ISSUED",
          tenant_created: true,
          tenant_reused: false,
        },
      ],
      pairing_issues: [
        {
          batch_id: "d436c224-5df5-4b4d-a772-1897f9147400",
          source_company_id: "sa-browser-v1-1234",
          tenant_id: "existing-tenant-1",
          pairing: {
            pairing_id: "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
            pairing_token: "relay-a-not-rendered",
            expires_at: "2026-08-27T15:10:00Z",
          },
        },
        {
          batch_id: "d436c224-5df5-4b4d-a772-1897f9147400",
          source_company_id: "sa-browser-v1-9876",
          tenant_id: "created-tenant-2",
          pairing: {
            pairing_id: "1a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
            pairing_token: "relay-b-not-rendered",
            expires_at: "2026-08-27T15:10:00Z",
          },
        },
      ],
      reused: false,
    });
    const pairingClaimFetch = vi.fn(async () => {
      return {
        ok: true,
        json: async () => ({ status: "CLAIMED" }),
      } as Response;
    });
    vi.stubGlobal("fetch", pairingClaimFetch);
    const batchEvents: Array<Record<string, unknown>> = [];
    const observeBatch = (event: MessageEvent<Record<string, unknown>>) => {
      if (
        event.data?.type ===
        "open-accounting.smartaccounts-browser-pairing-batch-issued.v1"
      )
        batchEvents.push(event.data);
    };
    window.addEventListener("message", observeBatch);
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await handoffVisibleCompanyCatalog();
    expect(
      (
        screen.getByLabelText(
          "All 2 relay-observed companies",
        ) as HTMLInputElement
      ).checked,
    ).toBe(false);
    await fireEvent.click(
      screen.getByLabelText("All 2 relay-observed companies"),
    );
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Create/reuse and pair all 2 companies",
      }),
    );
    await waitFor(() =>
      expect(
        apiMock.startSmartAccountsBrowserOnboardingBatch,
      ).toHaveBeenCalledWith({
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        mode: "all",
        selected_source_ids: ["sa-browser-v1-1234", "sa-browser-v1-9876"],
        owner_confirmed: true,
      }),
    );
    expect(
      window.sessionStorage.getItem(
        "open-accounting:smartaccounts-browser-onboarding-batch:v1",
      ),
    ).toBe("d436c224-5df5-4b4d-a772-1897f9147400");
    expect(apiMock.getMyTenants).not.toHaveBeenCalled();
    expect(apiMock.createTenant).not.toHaveBeenCalled();
    expect(apiMock.createSmartAccountsBrowserPairing).not.toHaveBeenCalled();
    await waitFor(() => expect(batchEvents).toHaveLength(1));
    expect(batchEvents[0]).toMatchObject({
      source: "open-accounting",
      version: 1,
      pairings: [
        { source_company_id: "sa-browser-v1-1234" },
        { source_company_id: "sa-browser-v1-9876" },
      ],
    });
    apiMock.getSmartAccountsBrowserOnboardingBatch.mockResolvedValue({
      batch: {
        batch_id: "d436c224-5df5-4b4d-a772-1897f9147400",
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        relay_observed_at: "2026-08-28T10:00:01Z",
        mode: "all",
        selected_sources: [
          {
            source_company_id: "sa-browser-v1-1234",
            source_company_name: "Company A",
          },
          {
            source_company_id: "sa-browser-v1-9876",
            source_company_name: "Company B",
          },
        ],
        observed_source_ids: ["sa-browser-v1-1234", "sa-browser-v1-9876"],
        observed_sources_sha256: "c".repeat(64),
        manifest_sha256: "d".repeat(64),
        status: "READY",
        created_at: "2026-08-28T10:00:02Z",
        updated_at: "2026-08-28T10:01:02Z",
      },
      outcomes: [
        {
          source_company_id: "sa-browser-v1-1234",
          source_company_name: "Company A",
          tenant_id: "existing-tenant-1",
          tenant_name: "Company A",
          pairing_id: "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
          status: "PAIRED",
          tenant_created: false,
          tenant_reused: true,
        },
        {
          source_company_id: "sa-browser-v1-9876",
          source_company_name: "Company B",
          tenant_id: "created-tenant-2",
          tenant_name: "Company B",
          pairing_id: "1a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
          status: "PAIRED",
          tenant_created: true,
          tenant_reused: false,
        },
      ],
      pairing_issues: [],
      reused: true,
    });
    apiMock.resumeSmartAccountsBrowserOnboardingBatch.mockImplementation(
      async () => apiMock.getSmartAccountsBrowserOnboardingBatch(),
    );
    apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow.mockResolvedValue({
      workflow: {
        batch_id: "d436c224-5df5-4b4d-a772-1897f9147400",
        schema_version: "smartaccounts-browser-batch-workflow-v1",
        history_from: "2024-01-01",
        header_probe_consent_confirmed: false,
        preparatory_manifest_sha256: "a".repeat(64),
        preparatory_consented_at: "2026-08-28T10:00:00Z",
        created_at: "2026-08-28T10:00:00Z",
        updated_at: "2026-08-28T10:01:02Z",
      },
      status: "DISCOVERY_REQUIRED",
      sources: [
        {
          batch_id: "d436c224-5df5-4b4d-a772-1897f9147400",
          source_company_id: "sa-browser-v1-1234",
          tenant_id: "existing-tenant-1",
          ordinal: 0,
          phase: "DISCOVERY_REQUIRED",
          phase_generation: 1,
          attempt_count: 0,
          created_at: "2026-08-28T10:00:00Z",
          updated_at: "2026-08-28T10:01:02Z",
        },
        {
          batch_id: "d436c224-5df5-4b4d-a772-1897f9147400",
          source_company_id: "sa-browser-v1-9876",
          tenant_id: "created-tenant-2",
          ordinal: 1,
          phase: "DISCOVERY_REQUIRED",
          phase_generation: 1,
          attempt_count: 0,
          created_at: "2026-08-28T10:00:00Z",
          updated_at: "2026-08-28T10:01:02Z",
        },
      ],
    });
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: {
          source: "smartaccounts-browser-relay",
          type: "smartaccounts-browser-relay.pairing-result.v1",
          pairing_id: "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
          status: "claimed",
        },
      }),
    );
    expect(
      apiMock.getSmartAccountsBrowserOnboardingBatch,
    ).not.toHaveBeenCalled();
    window.dispatchEvent(
      new MessageEvent("message", {
        source: window,
        origin: window.location.origin,
        data: {
          source: "smartaccounts-browser-relay",
          type: "smartaccounts-browser-relay.pairing-result.v1",
          pairing_id: "1a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
          status: "awaiting_browser",
        },
      }),
    );
    await waitFor(() => expect(pairingClaimFetch).toHaveBeenCalledTimes(1));
    expect(pairingClaimFetch).toHaveBeenCalledWith(
      expect.stringMatching(
        /\/api\/v1\/smartaccounts-browser-pairings\/[0-9a-f-]{36}\/claim$/,
      ),
      expect.objectContaining({
        method: "POST",
        credentials: "omit",
        cache: "no-store",
        redirect: "error",
        referrerPolicy: "no-referrer",
        headers: {
          Authorization: "Bearer relay-b-not-rendered",
          "Content-Type": "application/json",
        },
        body: JSON.stringify({
          source_company_id: "sa-browser-v1-9876",
        }),
      }),
    );
    await waitFor(() =>
      expect(
        apiMock.resumeSmartAccountsBrowserOnboardingBatch,
      ).toHaveBeenCalledWith("d436c224-5df5-4b4d-a772-1897f9147400"),
    );
    await waitFor(() =>
      expect(
        apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow,
      ).toHaveBeenCalledWith("d436c224-5df5-4b4d-a772-1897f9147400"),
    );
    expect(
      await screen.findByRole("heading", {
        name: "Selected/all company migration runner",
      }),
    ).toBeInTheDocument();
    expect(
      apiMock.startSmartAccountsBrowserCaptureWorkflow,
    ).not.toHaveBeenCalled();
    expect(apiMock.applySmartAccountsPackage).not.toHaveBeenCalled();
    expect(screen.queryByText("relay-a-not-rendered")).not.toBeInTheDocument();
    expect(screen.queryByText("relay-b-not-rendered")).not.toBeInTheDocument();
    window.removeEventListener("message", observeBatch);
  });

  it("restores only an opaque selected/all batch checkpoint after a pre-tenant page reload", async () => {
    const batchID = "d436c224-5df5-4b4d-a772-1897f9147400";
    window.sessionStorage.setItem(
      "open-accounting:smartaccounts-browser-onboarding-batch:v1",
      batchID,
    );
    apiMock.getSmartAccountsBrowserOnboardingBatch.mockResolvedValue({
      batch: {
        batch_id: batchID,
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        relay_observed_at: "2026-08-28T10:00:01Z",
        mode: "selected",
        selected_sources: [
          {
            source_company_id: "sa-browser-v1-1234",
            source_company_name: "Hold My Beer OÜ",
          },
        ],
        observed_source_ids: ["sa-browser-v1-1234"],
        observed_sources_sha256: "c".repeat(64),
        manifest_sha256: "d".repeat(64),
        status: "READY",
        created_at: "2026-08-28T10:00:02Z",
        updated_at: "2026-08-28T10:00:02Z",
      },
      outcomes: [
        {
          source_company_id: "sa-browser-v1-1234",
          source_company_name: "Hold My Beer OÜ",
          tenant_id: "tenant-hmb",
          tenant_name: "Hold My Beer OÜ",
          status: "PAIRED",
          tenant_created: false,
          tenant_reused: true,
        },
      ],
      pairing_issues: [],
      reused: true,
    });
    apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow.mockResolvedValue({
      workflow: {
        batch_id: batchID,
        schema_version: "smartaccounts-browser-batch-workflow-v1",
        history_from: "2024-01-01",
        header_probe_consent_confirmed: false,
        preparatory_manifest_sha256: "a".repeat(64),
        preparatory_consented_at: "2026-08-28T10:00:00Z",
        created_at: "2026-08-28T10:00:00Z",
        updated_at: "2026-08-28T10:00:00Z",
      },
      status: "DISCOVERY_REQUIRED",
      sources: [
        {
          batch_id: batchID,
          source_company_id: "sa-browser-v1-1234",
          tenant_id: "tenant-hmb",
          ordinal: 0,
          phase: "DISCOVERY_REQUIRED",
          phase_generation: 1,
          attempt_count: 0,
          created_at: "2026-08-28T10:00:00Z",
          updated_at: "2026-08-28T10:00:00Z",
        },
      ],
    });

    render(SmartAccountsSyncControl, { tenantId: "" });

    expect(
      await screen.findByRole("heading", {
        name: "Selected/all company batch",
      }),
    ).toBeInTheDocument();
    expect(apiMock.getSmartAccountsBrowserOnboardingBatch).toHaveBeenCalledWith(
      batchID,
    );
    expect(
      apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow,
    ).toHaveBeenCalledWith(batchID);
    expect(
      screen.getByText(/Restored the same immutable selected\/all batch/),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("link", { name: "Continue with this tenant" }),
    ).toHaveAttribute("href", "/migration?tenant=tenant-hmb");
    expect(
      window.sessionStorage.getItem(
        "open-accounting:smartaccounts-browser-onboarding-batch:v1",
      ),
    ).toBe(batchID);
    expect(
      screen.queryByText(
        /catalog-token-not-rendered|capture-token-not-rendered|pairing_token/i,
      ),
    ).not.toBeInTheDocument();
  });

  it("continues a restored pairing batch once, then hydrates only the exact ready workflow", async () => {
    const batchID = "d436c224-5df5-4b4d-a772-1897f9147400";
    window.sessionStorage.setItem(
      "open-accounting:smartaccounts-browser-onboarding-batch:v1",
      batchID,
    );
    apiMock.getSmartAccountsBrowserOnboardingBatch.mockResolvedValue({
      batch: {
        batch_id: batchID,
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        relay_observed_at: "2026-08-28T10:00:01Z",
        mode: "selected",
        selected_sources: [
          {
            source_company_id: "sa-browser-v1-1234",
            source_company_name: "Hold My Beer OÜ",
          },
        ],
        observed_source_ids: ["sa-browser-v1-1234"],
        observed_sources_sha256: "c".repeat(64),
        manifest_sha256: "d".repeat(64),
        status: "PENDING",
        created_at: "2026-08-28T10:00:02Z",
        updated_at: "2026-08-28T10:00:02Z",
      },
      outcomes: [
        {
          source_company_id: "sa-browser-v1-1234",
          source_company_name: "Hold My Beer OÜ",
          tenant_id: "tenant-hmb",
          tenant_name: "Hold My Beer OÜ",
          pairing_id: "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
          status: "PAIRING_ISSUED",
          tenant_created: false,
          tenant_reused: true,
        },
      ],
      pairing_issues: [],
      reused: true,
    });
    apiMock.resumeSmartAccountsBrowserOnboardingBatch.mockResolvedValue({
      batch: {
        batch_id: batchID,
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        relay_observed_at: "2026-08-28T10:00:01Z",
        mode: "selected",
        selected_sources: [
          {
            source_company_id: "sa-browser-v1-1234",
            source_company_name: "Hold My Beer OÜ",
          },
        ],
        observed_source_ids: ["sa-browser-v1-1234"],
        observed_sources_sha256: "c".repeat(64),
        manifest_sha256: "d".repeat(64),
        status: "READY",
        created_at: "2026-08-28T10:00:02Z",
        updated_at: "2026-08-28T10:01:02Z",
      },
      outcomes: [
        {
          source_company_id: "sa-browser-v1-1234",
          source_company_name: "Hold My Beer OÜ",
          tenant_id: "tenant-hmb",
          tenant_name: "Hold My Beer OÜ",
          pairing_id: "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
          status: "PAIRED",
          tenant_created: false,
          tenant_reused: true,
        },
      ],
      pairing_issues: [],
      reused: true,
    });
    apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow.mockResolvedValue({
      workflow: {
        batch_id: batchID,
        schema_version: "smartaccounts-browser-batch-workflow-v1",
        history_from: "2024-01-01",
        header_probe_consent_confirmed: false,
        preparatory_manifest_sha256: "a".repeat(64),
        preparatory_consented_at: "2026-08-28T10:00:00Z",
        created_at: "2026-08-28T10:00:00Z",
        updated_at: "2026-08-28T10:01:02Z",
      },
      status: "DISCOVERY_REQUIRED",
      sources: [
        {
          batch_id: batchID,
          source_company_id: "sa-browser-v1-1234",
          tenant_id: "tenant-hmb",
          ordinal: 0,
          phase: "DISCOVERY_REQUIRED",
          phase_generation: 1,
          attempt_count: 0,
          created_at: "2026-08-28T10:00:00Z",
          updated_at: "2026-08-28T10:01:02Z",
        },
      ],
    });

    render(SmartAccountsSyncControl, { tenantId: "" });
    await screen.findByText("Relay ready — SmartAccounts is signed in.");
    await screen.findByRole("button", { name: "Continue same pairing batch" });
    await fireEvent.click(
      screen.getByLabelText(
        /I confirm continuing the same immutable selected\/all pairing batch/,
      ),
    );
    await fireEvent.click(
      screen.getByRole("button", { name: "Continue same pairing batch" }),
    );

    await waitFor(() =>
      expect(
        apiMock.resumeSmartAccountsBrowserOnboardingBatch,
      ).toHaveBeenCalledWith(batchID),
    );
    await waitFor(() =>
      expect(
        apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow,
      ).toHaveBeenCalledWith(batchID),
    );
    expect(
      screen.getByRole("button", { name: "Continue safe workflow" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Refresh safe batch progress" }),
    ).not.toBeInTheDocument();
    expect(
      apiMock.acquireSmartAccountsBrowserOnboardingBatchCapture,
    ).not.toHaveBeenCalled();
  });

  it("opens the serial 082 runner only after a completed 081 batch, leaving source transfer and GL apply unstarted", async () => {
    apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow.mockRejectedValueOnce(
      new Error("workflow has not been prepared"),
    );
    apiMock.startSmartAccountsBrowserOnboardingBatch.mockResolvedValue({
      batch: {
        batch_id: "d436c224-5df5-4b4d-a772-1897f9147400",
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        relay_observed_at: "2026-08-28T10:00:01Z",
        mode: "all",
        selected_sources: [
          {
            source_company_id: "sa-browser-v1-1234",
            source_company_name: "Company A",
          },
          {
            source_company_id: "sa-browser-v1-9876",
            source_company_name: "Company B",
          },
        ],
        observed_source_ids: ["sa-browser-v1-1234", "sa-browser-v1-9876"],
        observed_sources_sha256: "c".repeat(64),
        manifest_sha256: "d".repeat(64),
        status: "READY",
        created_at: "2026-08-28T10:00:02Z",
        updated_at: "2026-08-28T10:00:02Z",
      },
      outcomes: [
        {
          source_company_id: "sa-browser-v1-1234",
          source_company_name: "Company A",
          tenant_id: "tenant-company-a",
          tenant_name: "Company A",
          status: "PAIRED",
          tenant_created: false,
          tenant_reused: true,
        },
        {
          source_company_id: "sa-browser-v1-9876",
          source_company_name: "Company B",
          tenant_id: "tenant-company-b",
          tenant_name: "Company B",
          status: "PAIRED",
          tenant_created: true,
          tenant_reused: false,
        },
      ],
      pairing_issues: [],
      reused: false,
    });
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await handoffVisibleCompanyCatalog();
    await fireEvent.click(
      screen.getByLabelText("All 2 relay-observed companies"),
    );
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Create/reuse and pair all 2 companies",
      }),
    );

    await screen.findByRole("heading", {
      name: "Selected/all company migration runner",
    });
    expect(
      screen.getByRole("button", { name: "Prepare safe batch workflow" }),
    ).toBeInTheDocument();
    expect(
      apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow,
    ).toHaveBeenCalledWith("d436c224-5df5-4b4d-a772-1897f9147400");
    expect(
      screen.queryByRole("button", { name: "Refresh safe batch progress" }),
    ).not.toBeInTheDocument();
    expect(
      apiMock.startSmartAccountsBrowserCaptureWorkflow,
    ).not.toHaveBeenCalled();
    expect(apiMock.applySmartAccountsPackage).not.toHaveBeenCalled();
    expect(
      screen.queryByText(/capture-token|pairing_token/i),
    ).not.toBeInTheDocument();
  });

  it("automatically refreshes a pending paired batch and hydrates its exact workflow without a manual reload", async () => {
    const batchID = "d436c224-5df5-4b4d-a772-1897f9147400";
    apiMock.getSmartAccountsBrowserOnboardingBatch.mockResolvedValue({
      batch: {
        batch_id: batchID,
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        relay_observed_at: "2026-08-28T10:00:01Z",
        mode: "all",
        selected_sources: [
          {
            source_company_id: "sa-browser-v1-1234",
            source_company_name: "Company A",
          },
          {
            source_company_id: "sa-browser-v1-9876",
            source_company_name: "Company B",
          },
        ],
        observed_source_ids: ["sa-browser-v1-1234", "sa-browser-v1-9876"],
        observed_sources_sha256: "c".repeat(64),
        manifest_sha256: "d".repeat(64),
        status: "READY",
        created_at: "2026-08-28T10:00:02Z",
        updated_at: "2026-08-28T10:01:02Z",
      },
      outcomes: [
        {
          source_company_id: "sa-browser-v1-1234",
          source_company_name: "Company A",
          tenant_id: "tenant-company-a",
          tenant_name: "Company A",
          status: "PAIRED",
          tenant_created: false,
          tenant_reused: true,
        },
        {
          source_company_id: "sa-browser-v1-9876",
          source_company_name: "Company B",
          tenant_id: "tenant-company-b",
          tenant_name: "Company B",
          status: "PAIRED",
          tenant_created: true,
          tenant_reused: false,
        },
      ],
      pairing_issues: [],
      reused: true,
    });
    apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow.mockResolvedValue({
      workflow: {
        batch_id: batchID,
        schema_version: "smartaccounts-browser-batch-workflow-v1",
        history_from: "2024-01-01",
        header_probe_consent_confirmed: false,
        preparatory_manifest_sha256: "a".repeat(64),
        preparatory_consented_at: "2026-08-28T10:00:00Z",
        created_at: "2026-08-28T10:00:00Z",
        updated_at: "2026-08-28T10:01:02Z",
      },
      status: "DISCOVERY_REQUIRED",
      sources: [
        {
          batch_id: batchID,
          source_company_id: "sa-browser-v1-1234",
          tenant_id: "tenant-company-a",
          ordinal: 0,
          phase: "DISCOVERY_REQUIRED",
          phase_generation: 1,
          attempt_count: 0,
          created_at: "2026-08-28T10:00:00Z",
          updated_at: "2026-08-28T10:01:02Z",
        },
        {
          batch_id: batchID,
          source_company_id: "sa-browser-v1-9876",
          tenant_id: "tenant-company-b",
          ordinal: 1,
          phase: "DISCOVERY_REQUIRED",
          phase_generation: 1,
          attempt_count: 0,
          created_at: "2026-08-28T10:00:00Z",
          updated_at: "2026-08-28T10:01:02Z",
        },
      ],
    });
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await handoffVisibleCompanyCatalog();
    vi.useFakeTimers();
    await fireEvent.click(
      screen.getByLabelText("All 2 relay-observed companies"),
    );
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Create/reuse and pair all 2 companies",
      }),
    );
    await vi.advanceTimersByTimeAsync(1_000);

    expect(apiMock.getSmartAccountsBrowserOnboardingBatch).toHaveBeenCalledWith(
      batchID,
    );
    expect(
      apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow,
    ).toHaveBeenCalledWith(batchID);
    expect(
      screen.getByRole("button", { name: "Continue safe workflow" }),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Refresh safe batch progress" }),
    ).not.toBeInTheDocument();
    expect(
      apiMock.acquireSmartAccountsBrowserOnboardingBatchCapture,
    ).not.toHaveBeenCalled();
  });

  it("serializes the first 082 metadata-only discovery through the server without starting a capture", async () => {
    const batchID = "d436c224-5df5-4b4d-a772-1897f9147400";
    const source = {
      batch_id: batchID,
      source_company_id: "sa-browser-v1-1234",
      tenant_id: "tenant-company-a",
      ordinal: 0,
      phase: "DISCOVERY_REQUIRED",
      phase_generation: 1,
      attempt_count: 0,
      created_at: "2026-08-28T10:00:00Z",
      updated_at: "2026-08-28T10:00:00Z",
    };
    const prepared = {
      workflow: {
        batch_id: batchID,
        schema_version: "smartaccounts-browser-batch-workflow-v1",
        history_from: "2024-01-01",
        header_probe_consent_confirmed: true,
        preparatory_manifest_sha256: "a".repeat(64),
        preparatory_consented_at: "2026-08-28T10:00:00Z",
        created_at: "2026-08-28T10:00:00Z",
        updated_at: "2026-08-28T10:00:00Z",
      },
      status: "DISCOVERY_REQUIRED",
      sources: [source],
    };
    apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow.mockRejectedValueOnce(
      new Error("workflow has not been prepared"),
    );
    apiMock.startSmartAccountsBrowserOnboardingBatch.mockResolvedValue({
      batch: {
        batch_id: batchID,
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        relay_observed_at: "2026-08-28T10:00:01Z",
        mode: "all",
        selected_sources: [
          {
            source_company_id: "sa-browser-v1-1234",
            source_company_name: "Company A",
          },
          {
            source_company_id: "sa-browser-v1-9876",
            source_company_name: "Company B",
          },
        ],
        observed_source_ids: ["sa-browser-v1-1234", "sa-browser-v1-9876"],
        observed_sources_sha256: "c".repeat(64),
        manifest_sha256: "d".repeat(64),
        status: "READY",
        created_at: "2026-08-28T10:00:02Z",
        updated_at: "2026-08-28T10:00:02Z",
      },
      outcomes: [
        {
          source_company_id: "sa-browser-v1-1234",
          source_company_name: "Company A",
          tenant_id: "tenant-company-a",
          tenant_name: "Company A",
          status: "PAIRED",
          tenant_created: false,
          tenant_reused: true,
        },
        {
          source_company_id: "sa-browser-v1-9876",
          source_company_name: "Company B",
          tenant_id: "tenant-company-b",
          tenant_name: "Company B",
          status: "PAIRED",
          tenant_created: true,
          tenant_reused: false,
        },
      ],
      pairing_issues: [],
      reused: false,
    });
    apiMock.prepareSmartAccountsBrowserOnboardingBatchWorkflow.mockResolvedValue(
      prepared,
    );
    apiMock.acquireSmartAccountsBrowserOnboardingBatchDiscovery.mockResolvedValue(
      {
        source: {
          ...source,
          phase: "DISCOVERY_RUNNING",
          phase_generation: 2,
          lease_id: "317f6fec-1994-4cfe-8ea6-bb7281d3050f",
        },
        discovery: {
          discovery_id: "417f6fec-1994-4cfe-8ea6-bb7281d3050f",
          tenant_id: "tenant-company-a",
          source_company_id: "sa-browser-v1-1234",
          manifest_version: "smartaccounts-brave-ui-v2",
          resource_ids: ["general_ledger"],
          expires_at: "2026-08-28T10:10:00Z",
          discovery_consent: {
            version: 1,
            confirmed: true,
            confirmed_at: "2026-08-28T10:00:00Z",
            scope: "metadata_only",
          },
        },
      },
    );
    apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow.mockResolvedValue({
      ...prepared,
      status: "DISCOVERY_RUNNING",
      sources: [
        {
          ...source,
          phase: "DISCOVERY_RUNNING",
          phase_generation: 2,
          lease_expires_at: "2026-08-28T10:10:00Z",
        },
      ],
    });
    const discoveryEvents: Array<Record<string, unknown>> = [];
    const observeDiscovery = (event: MessageEvent<Record<string, unknown>>) => {
      if (
        event.data?.type ===
        "open-accounting.smartaccounts-browser-discovery-issued.v1"
      )
        discoveryEvents.push(event.data);
    };
    window.addEventListener("message", observeDiscovery);
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await handoffVisibleCompanyCatalog();
    await fireEvent.click(
      screen.getByLabelText("All 2 relay-observed companies"),
    );
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Create/reuse and pair all 2 companies",
      }),
    );
    await screen.findByRole("heading", {
      name: "Selected/all company migration runner",
    });
    await fireEvent.input(screen.getByLabelText("History starts"), {
      target: { value: "2024-01-01" },
    });
    await fireEvent.click(
      screen.getByLabelText(
        /I approve metadata-only discovery for every paired company/,
      ),
    );
    await fireEvent.click(
      screen.getByLabelText(
        /I separately approve bounded CSV header-name probing/,
      ),
    );
    await fireEvent.click(
      screen.getByLabelText(/I confirm this selected\/all batch is ready/),
    );
    await fireEvent.click(
      screen.getByRole("button", { name: "Prepare safe batch workflow" }),
    );

    await waitFor(() =>
      expect(
        apiMock.prepareSmartAccountsBrowserOnboardingBatchWorkflow,
      ).toHaveBeenCalledWith(
        batchID,
        expect.objectContaining({
          history_from: "2024-01-01",
          owner_confirmed: true,
        }),
      ),
    );
    await waitFor(() =>
      expect(
        apiMock.acquireSmartAccountsBrowserOnboardingBatchDiscovery,
      ).toHaveBeenCalledWith(batchID, {
        metadata_only_consent_confirmed: true,
        response_header_probe_confirmed: true,
      }),
    );
    await waitFor(() => expect(discoveryEvents).toHaveLength(1));
    expect(discoveryEvents[0]).toMatchObject({
      source: "open-accounting",
      discovery_id: "417f6fec-1994-4cfe-8ea6-bb7281d3050f",
      source_company_id: "sa-browser-v1-1234",
    });
    expect(JSON.stringify(discoveryEvents[0])).not.toContain("capture_token");
    expect(
      apiMock.acquireSmartAccountsBrowserOnboardingBatchCapture,
    ).not.toHaveBeenCalled();
    expect(apiMock.applySmartAccountsPackage).not.toHaveBeenCalled();
    window.removeEventListener("message", observeDiscovery);
  });

  it("shows an isolated partial batch failure without preventing other selected companies from pairing", async () => {
    apiMock.startSmartAccountsBrowserOnboardingBatch.mockResolvedValue({
      batch: {
        batch_id: "d436c224-5df5-4b4d-a772-1897f9147400",
        catalog_receipt_id: "b436c224-5df5-4b4d-a772-1897f9147400",
        relay_observed_at: "2026-08-28T10:00:01Z",
        mode: "all",
        selected_sources: [
          {
            source_company_id: "sa-browser-v1-1234",
            source_company_name: "Company A",
          },
          {
            source_company_id: "sa-browser-v1-9876",
            source_company_name: "Company B",
          },
        ],
        observed_source_ids: ["sa-browser-v1-1234", "sa-browser-v1-9876"],
        observed_sources_sha256: "c".repeat(64),
        manifest_sha256: "d".repeat(64),
        status: "REVIEW_REQUIRED",
        created_at: "2026-08-28T10:00:02Z",
        updated_at: "2026-08-28T10:00:02Z",
      },
      outcomes: [
        {
          source_company_id: "sa-browser-v1-1234",
          source_company_name: "Company A",
          tenant_id: "existing-tenant-1",
          tenant_name: "Company A",
          pairing_id: "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
          status: "PAIRING_ISSUED",
          tenant_created: false,
          tenant_reused: true,
        },
        {
          source_company_id: "sa-browser-v1-9876",
          source_company_name: "Company B",
          status: "FAILED",
          tenant_created: false,
          tenant_reused: false,
          reason_code: "tenant_create_failed",
        },
      ],
      pairing_issues: [
        {
          batch_id: "d436c224-5df5-4b4d-a772-1897f9147400",
          source_company_id: "sa-browser-v1-1234",
          tenant_id: "existing-tenant-1",
          pairing: {
            pairing_id: "0a2fa9cd-1e5d-4f4d-9ee8-4a08de7310d3",
            pairing_token: "relay-a-not-rendered",
            expires_at: "2026-08-27T15:10:00Z",
          },
        },
      ],
      reused: false,
    });
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await handoffVisibleCompanyCatalog();
    await fireEvent.click(
      screen.getByLabelText("All 2 relay-observed companies"),
    );
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Create/reuse and pair all 2 companies",
      }),
    );
    await screen.findByText("Company B: failed (tenant_create_failed)");
    expect(
      screen.getByText(/other companies can continue/),
    ).toBeInTheDocument();
    expect(apiMock.getMyTenants).not.toHaveBeenCalled();
    expect(apiMock.createTenant).not.toHaveBeenCalled();
    expect(apiMock.createSmartAccountsBrowserPairing).not.toHaveBeenCalled();
  });

  it("lets the bridge derive the source identity and clears the opaque credential reference after connection", async () => {
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });

    expect(
      screen.getByText("Derived from the signed-in Brave session"),
    ).toBeInTheDocument();
    const sourceCredentialReferenceInput = screen.getByLabelText(
      "External credential reference",
    ) as HTMLInputElement;
    await fireEvent.input(sourceCredentialReferenceInput, {
      target: { value: "secret-ref://file/connection-test" },
    });
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Connect external reference & start full-history capture",
      }),
    );

    await waitFor(() => {
      expect(apiMock.configureSmartAccountsSync).toHaveBeenCalledWith(
        "tenant-1",
        {
          source_credential_reference: "secret-ref://file/connection-test",
          smartaccounts_gl_authoritative: true,
          invoice_payment_mode: "NON_POSTING",
        },
      );
    });
    await waitFor(() =>
      expect(apiMock.requestSmartAccountsSyncDryRun).toHaveBeenCalledWith(
        "tenant-1",
        "sa-company-hmb-9881",
        { scope_mode: "full_history" },
      ),
    );
    expect(sourceCredentialReferenceInput.value).toBe("");
    expect(screen.getByText("Hold My Beer OÜ")).toBeInTheDocument();
  });

  it("clears submitted external credential references and redacts them from a failed bridge response", async () => {
    apiMock.configureSmartAccountsSync.mockRejectedValue(
      new Error("SmartAccounts bridge connection or validation failed"),
    );
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });

    await waitFor(() =>
      expect(
        screen.getByLabelText("External credential reference"),
      ).toBeInTheDocument(),
    );
    const sourceCredentialReferenceInput = screen.getByLabelText(
      "External credential reference",
    ) as HTMLInputElement;
    await fireEvent.input(sourceCredentialReferenceInput, {
      target: { value: "secret-ref://file/do-not-echo" },
    });
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Connect external reference & start full-history capture",
      }),
    );

    await waitFor(() =>
      expect(screen.getByRole("alert")).toHaveTextContent(
        "connection or validation failed",
      ),
    );
    expect(sourceCredentialReferenceInput.value).toBe("");
    expect(screen.getByRole("alert")).not.toHaveTextContent(
      "secret-ref://file/do-not-echo",
    );
  });

  it("shows only safe capture progress returned from the control plane", async () => {
    apiMock.requestSmartAccountsSyncDryRun.mockResolvedValue(
      syncStatus({
        configured: true,
        secret_reference_configured: true,
        capture_status: "running",
        capture_run_id: "capture-test-1",
        capture_progress: {
          run_id: "capture-test-1",
          status: "running",
          scope_mode: "full_history",
          source_as_of_date: "2026-08-27",
          cutoff_at: "2026-08-27T12:00:00Z",
          resources: [
            {
              resource_id: "accounts",
              endpoint_status: "ok",
              status: "completed",
              page_count: 1,
            },
          ],
          summary: {
            total: 2,
            completed: 1,
            running: 1,
            interrupted: 0,
            rate_limited: 0,
            review_required: 0,
            dependency_required: 0,
            brave_discovery_required: 0,
          },
        },
      }),
    );
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await fireEvent.input(
      screen.getByLabelText("External credential reference"),
      { target: { value: "secret-ref://file/connection-test" } },
    );
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Connect external reference & start full-history capture",
      }),
    );

    await waitFor(() =>
      expect(
        screen.getByText(
          "Capture run capture-test-1: 1/2 resources complete (full history through 2026-08-27).",
        ),
      ).toBeInTheDocument(),
    );
    expect(screen.getByText("accounts: completed")).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: /Start full-history capture/ }),
    ).not.toBeInTheDocument();
  });

  it("captures only the required date-window services without replacing the full-history package", async () => {
    const fullHistory = {
      run_id: "capture-full",
      status: "completed_with_review",
      scope_mode: "full_history" as const,
      source_as_of_date: "2026-08-27",
      cutoff_at: "2026-08-27T12:00:00Z",
      resource_ids: [
        "general.entries.get",
        "inventory.warehouse_movements.get",
      ],
      resources: [
        {
          resource_id: "general.entries.get",
          endpoint_status: "documented",
          status: "completed",
        },
        {
          resource_id: "inventory.warehouse_movements.get",
          endpoint_status: "documented",
          status: "review_required",
          reason_code: "source_date_window_required",
        },
      ],
      summary: {
        total: 2,
        completed: 1,
        running: 0,
        interrupted: 0,
        rate_limited: 0,
        review_required: 1,
        dependency_required: 0,
        brave_discovery_required: 0,
      },
      staging: {
        package_id: "package-full",
        package_sha256: "a".repeat(64),
        status: "staged_review_required",
        record_chunks_acknowledged: 1,
        artifact_chunks_acknowledged: 0,
        finalized: true,
      },
    };
    const dateWindow = {
      run_id: "capture-window",
      status: "running",
      scope_mode: "window" as const,
      date_from: "2020-01-01",
      date_to: "2026-08-27",
      source_as_of_date: "2026-08-27",
      cutoff_at: "2026-08-27T12:05:00Z",
      resource_ids: ["inventory.warehouse_movements.get"],
      resources: [
        {
          resource_id: "inventory.warehouse_movements.get",
          endpoint_status: "documented",
          status: "running",
        },
      ],
      summary: {
        total: 1,
        completed: 0,
        running: 1,
        interrupted: 0,
        rate_limited: 0,
        review_required: 0,
        dependency_required: 0,
        brave_discovery_required: 0,
      },
    };
    apiMock.requestSmartAccountsSyncDryRun
      .mockResolvedValueOnce(
        syncStatus({
          configured: true,
          secret_reference_configured: true,
          capture_progress: fullHistory,
          capture_progresses: [fullHistory],
        }),
      )
      .mockResolvedValueOnce(
        syncStatus({
          configured: true,
          secret_reference_configured: true,
          capture_progress: dateWindow,
          capture_progresses: [dateWindow, fullHistory],
        }),
      );
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await fireEvent.input(
      screen.getByLabelText("External credential reference"),
      { target: { value: "secret-ref://file/connection-test" } },
    );
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Connect external reference & start full-history capture",
      }),
    );
    await screen.findByText(/SmartAccounts requires a date range/);
    await fireEvent.input(screen.getByLabelText("From"), {
      target: { value: "2020-01-01" },
    });
    await fireEvent.input(screen.getByLabelText("To"), {
      target: { value: "2026-08-27" },
    });
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Capture missing date-window services",
      }),
    );
    await waitFor(() =>
      expect(apiMock.requestSmartAccountsSyncDryRun).toHaveBeenLastCalledWith(
        "tenant-1",
        "sa-company-hmb-9881",
        {
          scope_mode: "window",
          date_from: "2020-01-01",
          date_to: "2026-08-27",
          resource_ids: ["inventory.warehouse_movements.get"],
        },
      ),
    );
    expect(screen.getByText(/Capture run capture-full/)).toBeInTheDocument();
  });

  it("requires each date-window follow-up to reach the full-capture source-as-of date", async () => {
    const fullHistory = {
      run_id: "capture-full",
      status: "completed_with_review",
      scope_mode: "full_history" as const,
      source_as_of_date: "2026-08-27",
      cutoff_at: "2026-08-27T12:00:00Z",
      resource_ids: ["inventory.warehouse_movements.get"],
      resources: [
        {
          resource_id: "inventory.warehouse_movements.get",
          endpoint_status: "documented",
          status: "review_required",
          reason_code: "source_date_window_required",
        },
      ],
      summary: {
        total: 1,
        completed: 0,
        running: 0,
        interrupted: 0,
        rate_limited: 0,
        review_required: 1,
        dependency_required: 0,
        brave_discovery_required: 0,
      },
      staging: {
        package_id: "package-full",
        package_sha256: "a".repeat(64),
        status: "staged_review_required",
        record_chunks_acknowledged: 1,
        artifact_chunks_acknowledged: 0,
        finalized: true,
      },
    };
    window.sessionStorage.setItem(
      "open-accounting:smartaccounts-source:tenant-1",
      "sa-company-hmb-9881",
    );
    apiMock.getSmartAccountsSyncStatus.mockResolvedValue(
      syncStatus({
        configured: true,
        secret_reference_configured: true,
        capture_progress: fullHistory,
        capture_progresses: [fullHistory],
      }),
    );
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await screen.findByText(/source-as-of date \(2026-08-27\)/);
    await fireEvent.input(screen.getByLabelText("From"), {
      target: { value: "2020-01-01" },
    });
    await fireEvent.input(screen.getByLabelText("To"), {
      target: { value: "2026-08-26" },
    });
    const capture = screen.getByRole("button", {
      name: "Capture missing date-window services",
    });
    expect(capture).toBeDisabled();
    await fireEvent.input(screen.getByLabelText("To"), {
      target: { value: "2026-08-27" },
    });
    expect(capture).toBeEnabled();
  });

  it("reopens a selected/all source with only bound IDs, reloads server state, and resolves policy only for the exact preview", async () => {
    const batchID = "d436c224-5df5-4b4d-a772-1897f9147400";
    const packageSHA = "f".repeat(64);
    const previewSHA = "a".repeat(64);
    apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow.mockResolvedValue({
      workflow: {
        batch_id: batchID,
        schema_version: "smartaccounts-browser-batch-workflow-v1",
        history_from: "2024-01-01",
        header_probe_consent_confirmed: true,
        preparatory_manifest_sha256: "b".repeat(64),
        preparatory_consented_at: "2026-08-28T10:00:00Z",
        created_at: "2026-08-28T10:00:00Z",
        updated_at: "2026-08-28T10:01:00Z",
      },
      status: "PREVIEW_READY",
      sources: [
        {
          batch_id: batchID,
          source_company_id: "sa-browser-v1-123456",
          tenant_id: "tenant-1",
          ordinal: 0,
          phase: "PREVIEW_READY",
          phase_generation: 7,
          attempt_count: 1,
          package_id: "package-1",
          package_sha256: packageSHA,
          preview_id: "preview-1",
          preview_sha256: previewSHA,
          created_at: "2026-08-28T10:00:00Z",
          updated_at: "2026-08-28T10:01:00Z",
        },
      ],
    });
    apiMock.getSmartAccountsPackageArchiveCoverage.mockResolvedValue({
      package_id: "package-1",
      package_sha256: packageSHA,
      manifest_sha256: "c".repeat(64),
      scope_mode: "partial",
      declared_record_count: 4,
      observed_record_count: 4,
      artifact_count: 2,
      integrity_ok: true,
      unconsumed_record_count: 3,
      review_required_record_count: 1,
      buckets: [],
    });
    apiMock.previewSmartAccountsPackage.mockResolvedValue({
      id: "preview-1",
      tenant_id: "tenant-1",
      package_id: "package-1",
      source_company_id: "sa-browser-v1-123456",
      status: "PREVIEW_READY",
      preview_sha256: previewSHA,
      financial_writes_planned: true,
      financial_writes_applied: false,
      journals: [{}],
      account_imports: [{}],
      non_posting_record_count: 3,
    });

    render(SmartAccountsSyncControl, {
      tenantId: "tenant-1",
      ownerContinuationBatchId: batchID,
      ownerContinuationSourceCompanyId: "sa-browser-v1-123456",
    });

    await screen.findByRole("heading", {
      name: "Selected/all source review and GL apply",
    });
    await waitFor(() =>
      expect(
        apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow,
      ).toHaveBeenCalledWith(batchID),
    );
    expect(apiMock.getSmartAccountsSyncStatus).not.toHaveBeenCalled();
    expect(readinessPings).toHaveLength(0);
    await waitFor(() =>
      expect(
        apiMock.getSmartAccountsPackageArchiveCoverage,
      ).toHaveBeenCalledWith("tenant-1", "package-1"),
    );
    expect(
      screen.getByText(
        /Count-only archive coverage: 4\/4 records, 2 artifacts, 1 review-required, 3 unconsumed; integrity verified/,
      ),
    ).toBeInTheDocument();
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Load current GL preview for review",
      }),
    );
    await waitFor(() =>
      expect(apiMock.previewSmartAccountsPackage).toHaveBeenCalledWith(
        "tenant-1",
        "package-1",
        { use_source_chart: true },
      ),
    );
    await fireEvent.click(
      screen.getByLabelText(
        /I reviewed this server-bound partial, GL-authoritative plan/,
      ),
    );
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Confirm and apply reviewed GL plan",
      }),
    );
    await waitFor(() =>
      expect(apiMock.resolveSmartAccountsTolerancePolicy).toHaveBeenCalledWith(
        "tenant-1",
        "sa-browser-v1-123456",
        { package_id: "package-1", preview_id: "preview-1" },
      ),
    );
    await waitFor(() =>
      expect(apiMock.applySmartAccountsPackage).toHaveBeenCalledWith(
        "tenant-1",
        {
          confirm: true,
          preview_id: "preview-1",
          preview_sha256: previewSHA,
          tolerance_policy_id: "policy-1",
        },
      ),
    );
    expect(
      screen.queryByText("server-derived-digest-never-rendered"),
    ).not.toBeInTheDocument();
    expect(window.sessionStorage.length).toBe(0);
  });

  it("fails closed when a selected/all continuation workflow does not bind the requested tenant and source", async () => {
    const batchID = "d436c224-5df5-4b4d-a772-1897f9147400";
    apiMock.getSmartAccountsBrowserOnboardingBatchWorkflow.mockResolvedValue({
      workflow: {
        batch_id: batchID,
        schema_version: "smartaccounts-browser-batch-workflow-v1",
        history_from: "2024-01-01",
        header_probe_consent_confirmed: true,
        preparatory_manifest_sha256: "b".repeat(64),
        preparatory_consented_at: "2026-08-28T10:00:00Z",
        created_at: "2026-08-28T10:00:00Z",
        updated_at: "2026-08-28T10:01:00Z",
      },
      status: "PREVIEW_READY",
      sources: [
        {
          batch_id: batchID,
          source_company_id: "sa-browser-v1-123456",
          tenant_id: "tenant-other",
          ordinal: 0,
          phase: "PREVIEW_READY",
          phase_generation: 7,
          attempt_count: 1,
          package_id: "package-1",
          package_sha256: "f".repeat(64),
          preview_id: "preview-1",
          preview_sha256: "a".repeat(64),
          created_at: "2026-08-28T10:00:00Z",
          updated_at: "2026-08-28T10:01:00Z",
        },
      ],
    });

    render(SmartAccountsSyncControl, {
      tenantId: "tenant-1",
      ownerContinuationBatchId: batchID,
      ownerContinuationSourceCompanyId: "sa-browser-v1-123456",
    });

    expect(await screen.findByRole("alert")).toHaveTextContent(
      /source continuation is unavailable|belongs to another tenant/i,
    );
    expect(
      apiMock.getSmartAccountsPackageArchiveCoverage,
    ).not.toHaveBeenCalled();
    expect(apiMock.previewSmartAccountsPackage).not.toHaveBeenCalled();
    expect(apiMock.applySmartAccountsPackage).not.toHaveBeenCalled();
    expect(readinessPings).toHaveLength(0);
  });

  it("previews a finalized safe package and resolves an independently approved policy only at the separate financial apply action", async () => {
    apiMock.requestSmartAccountsSyncDryRun.mockResolvedValue(
      syncStatus({
        source_company_id: "sa-browser-v1-123456",
        configured: true,
        secret_reference_configured: true,
        capture_progress: {
          run_id: "capture-1",
          status: "complete",
          scope_mode: "full_history",
          source_as_of_date: "2026-08-27",
          cutoff_at: "2026-08-27T12:00:00Z",
          resources: [],
          summary: {
            total: 1,
            completed: 1,
            running: 0,
            interrupted: 0,
            rate_limited: 0,
            review_required: 0,
            dependency_required: 0,
            brave_discovery_required: 0,
          },
          staging: {
            package_id: "package-1",
            package_sha256: "a".repeat(64),
            status: "staged_review_required",
            record_chunks_acknowledged: 2,
            artifact_chunks_acknowledged: 1,
            finalized: true,
          },
        },
      }),
    );
    render(SmartAccountsSyncControl, { tenantId: "tenant-1" });
    await fireEvent.input(
      screen.getByLabelText("External credential reference"),
      { target: { value: "secret-ref://file/connection-test" } },
    );
    await fireEvent.click(
      screen.getByRole("button", {
        name: "Connect external reference & start full-history capture",
      }),
    );
    await fireEvent.click(
      await screen.findByRole("button", {
        name: "Prepare chart and GL preview",
      }),
    );
    await waitFor(() =>
      expect(apiMock.previewSmartAccountsPackage).toHaveBeenCalledWith(
        "tenant-1",
        "package-1",
        { use_source_chart: true },
      ),
    );
    const apply = await screen.findByRole("button", {
      name: "Confirm and apply GL plan",
    });
    expect(apply).toBeDisabled();
    await fireEvent.click(
      screen.getByLabelText(/I reviewed this GL-authoritative plan/),
    );
    await fireEvent.click(apply);
    await waitFor(() =>
      expect(apiMock.resolveSmartAccountsTolerancePolicy).toHaveBeenCalledWith(
        "tenant-1",
        "sa-browser-v1-123456",
        { package_id: "package-1", preview_id: "preview-1" },
      ),
    );
    await waitFor(() =>
      expect(apiMock.applySmartAccountsPackage).toHaveBeenCalledWith(
        "tenant-1",
        {
          confirm: true,
          preview_id: "preview-1",
          preview_sha256: "a".repeat(64),
          tolerance_policy_id: "policy-1",
        },
      ),
    );
  });
});
