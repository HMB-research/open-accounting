import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import {
  cleanup,
  fireEvent,
  render,
  screen,
  waitFor,
} from "@testing-library/svelte";
import type {
  BundleValidationReport,
  MigrationExecutionPlan,
  MigrationExecutionRun,
  MigrationExecutionRunEvent,
  MigrationProviderPresetInfo,
} from "$lib/api";

const { apiMock } = vi.hoisted(() => ({
  apiMock: {
    validateMigrationBundle: vi.fn(),
    listMigrationProviderPresets: vi.fn(),
    planMigrationExecution: vi.fn(),
    executeMigration: vi.fn(),
    listMigrationExecutionRuns: vi.fn(),
    getMigrationExecutionRun: vi.fn(),
    watchMigrationExecutionRun: vi.fn(),
    issueSmartAccountsBrowserOnboardingCatalog: vi.fn(),
    getSmartAccountsBrowserOnboardingCatalog: vi.fn(),
    getSmartAccountsSyncStatus: vi.fn(),
    getSmartAccountsBrowserCaptureStatus: vi.fn(),
    getSmartAccountsBrowserCaptureWorkflowStatus: vi.fn(),
  },
}));

vi.mock("$lib/api", async () => {
  const actual = await vi.importActual<typeof import("$lib/api")>("$lib/api");
  return {
    ...actual,
    api: apiMock,
  };
});

import MigrationWorkbench from "$lib/components/MigrationWorkbench.svelte";

function validationReport(
  overrides: Partial<BundleValidationReport> = {},
): BundleValidationReport {
  return {
    summary: {
      files_validated: 1,
      rows_validated: 2,
      error_count: 0,
      warning_count: 0,
      ready: true,
    },
    files: [
      {
        kind: "contacts",
        file_name: "contacts.csv",
        rows: 2,
      },
    ],
    issues: [],
    remediation_actions: [],
    ...overrides,
  };
}

function executionPlan(): MigrationExecutionPlan {
  return {
    summary: {
      validation_ready: true,
      ready: true,
      step_count: 1,
      ready_step_count: 1,
      needs_context_count: 0,
      blocked_step_count: 0,
    },
    validation: validationReport(),
    steps: [
      {
        step_number: 1,
        kind: "contacts",
        file_name: "contacts.csv",
        status: "READY",
        message: "Ready to import.",
        action: "Import contacts.",
        api_method: "POST",
        api_path: "/api/v1/tenants/tenant-1/contacts/import",
        cli_command: "oa contacts import --file contacts.csv",
      },
    ],
    remediation_actions: [],
  };
}

function executionRun(
  overrides: Partial<MigrationExecutionRun> = {},
): MigrationExecutionRun {
  return {
    id: "run-1",
    tenant_id: "tenant-1",
    created_at: "2026-06-14T09:00:00Z",
    updated_at: "2026-06-14T09:01:00Z",
    summary: {
      status: "succeeded",
      confirmed: true,
      resumed: false,
      plan_ready: true,
      validation_ready: true,
      step_count: 1,
      running_step_count: 0,
      succeeded_step_count: 1,
      failed_step_count: 0,
      skipped_step_count: 0,
      planned_step_count: 0,
      resumed_step_count: 0,
      completed_step_count: 1,
      remaining_step_count: 0,
      progress_percent: 100,
      duration_ms: 1500,
      needs_context_count: 0,
      blocked_step_count: 0,
    },
    plan: executionPlan(),
    steps: [
      {
        step_number: 1,
        kind: "contacts",
        file_name: "contacts.csv",
        status: "SUCCEEDED",
        message: "Imported contacts.",
        started_at: "2026-06-14T09:00:00Z",
        completed_at: "2026-06-14T09:00:01.500Z",
        duration_ms: 1500,
        api_path: "/api/v1/tenants/tenant-1/contacts/import",
        cli_command: "oa contacts import --file contacts.csv",
      },
    ],
    remediation_actions: [],
    ...overrides,
  };
}

function runningExecutionRun(): MigrationExecutionRun {
  const base = executionRun();
  return {
    ...base,
    summary: {
      ...base.summary,
      status: "running",
      running_step_count: 1,
      succeeded_step_count: 1,
      completed_step_count: 1,
      remaining_step_count: 1,
      progress_percent: 50,
      active_step_number: 2,
      active_step_kind: "contacts",
      active_step_file_name: "contacts-next.csv",
      active_step_status: "RUNNING",
      active_step_started_at: "2026-06-14T09:00:02Z",
      step_count: 2,
    },
    steps: [
      ...(base.steps ?? []),
      {
        step_number: 2,
        kind: "contacts",
        file_name: "contacts-next.csv",
        status: "RUNNING",
        message: "Import running.",
        started_at: "2026-06-14T09:00:02Z",
        api_path: "/api/v1/tenants/tenant-1/contacts/import",
        cli_command: "oa contacts import --file contacts-next.csv",
      },
    ],
  };
}

function completedStreamingRun(): MigrationExecutionRun {
  const base = runningExecutionRun();
  return {
    ...base,
    updated_at: "2026-06-14T09:00:04Z",
    summary: {
      ...base.summary,
      status: "succeeded",
      running_step_count: 0,
      succeeded_step_count: 2,
      completed_step_count: 2,
      remaining_step_count: 0,
      progress_percent: 100,
      duration_ms: 4000,
      active_step_number: undefined,
      active_step_kind: undefined,
      active_step_file_name: undefined,
      active_step_status: undefined,
      active_step_started_at: undefined,
    },
    steps: [
      ...(base.steps ?? []).slice(0, 1),
      {
        step_number: 2,
        kind: "contacts",
        file_name: "contacts-next.csv",
        status: "SUCCEEDED",
        message: "Imported contacts-next.csv.",
        started_at: "2026-06-14T09:00:02Z",
        completed_at: "2026-06-14T09:00:04Z",
        duration_ms: 2000,
        api_path: "/api/v1/tenants/tenant-1/contacts/import",
        cli_command: "oa contacts import --file contacts-next.csv",
      },
    ],
  };
}

function providerPresets(): MigrationProviderPresetInfo[] {
  return [
    {
      preset: "generic",
      label: "Generic",
      description: "Uses canonical headers.",
      file_kind_count: 24,
      preset_alias_count: 0,
      file_kinds: [],
    },
    {
      preset: "merit",
      label: "Merit",
      description: "Adds Merit aliases.",
      file_kind_count: 24,
      preset_alias_count: 64,
      file_kinds: [
        {
          kind: "accounts",
          required_column_groups: [["code"], ["name"], ["account_type"]],
          preset_alias_count: 6,
          sample_aliases: [
            { source_header: "konto", canonical_header: "code" },
          ],
        },
      ],
    },
    {
      preset: "smartaccounts",
      label: "SmartAccounts",
      description: "Adds SmartAccounts aliases.",
      file_kind_count: 24,
      preset_alias_count: 58,
      file_kinds: [],
    },
    {
      preset: "directo",
      label: "Directo",
      description: "Adds Directo aliases.",
      file_kind_count: 24,
      preset_alias_count: 61,
      file_kinds: [
        {
          kind: "invoices",
          required_column_groups: [["invoice_number"], ["issue_date"]],
          preset_alias_count: 4,
          sample_aliases: [
            { source_header: "arve", canonical_header: "invoice_number" },
          ],
        },
      ],
    },
  ];
}

async function addContactsFile() {
  await fireEvent.change(screen.getByLabelText("File kind"), {
    target: { value: "contacts" },
  });
  await fireEvent.input(screen.getByLabelText("CSV or XML content"), {
    target: { value: "name\nAcme OU\n" },
  });
  await fireEvent.click(screen.getByRole("button", { name: "Add text file" }));
  await waitFor(() =>
    expect(screen.getByText("contacts.csv")).toBeInTheDocument(),
  );
}

describe("MigrationWorkbench", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    vi.clearAllMocks();
    apiMock.listMigrationExecutionRuns.mockResolvedValue([]);
    apiMock.listMigrationProviderPresets.mockResolvedValue(providerPresets());
    apiMock.validateMigrationBundle.mockResolvedValue(validationReport());
    apiMock.planMigrationExecution.mockResolvedValue(executionPlan());
    apiMock.executeMigration.mockResolvedValue(executionRun());
    apiMock.getMigrationExecutionRun.mockResolvedValue(executionRun());
    apiMock.watchMigrationExecutionRun.mockResolvedValue(undefined);
  });

  it("builds an execution plan from a pasted migration bundle", async () => {
    render(MigrationWorkbench, { tenantId: "tenant-1" });

    await waitFor(() =>
      expect(apiMock.listMigrationExecutionRuns).toHaveBeenCalledWith(
        "tenant-1",
        {
          status: undefined,
          limit: 10,
        },
      ),
    );
    await addContactsFile();
    await fireEvent.input(screen.getByLabelText("E-invoice invoice type"), {
      target: { value: "sales" },
    });
    await fireEvent.click(screen.getByRole("button", { name: "Build plan" }));

    await waitFor(() =>
      expect(apiMock.planMigrationExecution).toHaveBeenCalledTimes(1),
    );
    expect(apiMock.planMigrationExecution).toHaveBeenCalledWith("tenant-1", {
      files: [
        {
          kind: "contacts",
          file_name: "contacts.csv",
          csv_content: "name\nAcme OU\n",
        },
      ],
      provider_preset: "generic",
      e_invoice_contact_mode: "supplier",
      e_invoice_invoice_type: "sales",
    });
    expect(
      await screen.findByText("Migration execution plan is ready."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("oa contacts import --file contacts.csv"),
    ).toBeInTheDocument();
  });

  it("exposes the pre-tenant relay catalog flow without tenant-scoped status or capture calls", async () => {
    apiMock.issueSmartAccountsBrowserOnboardingCatalog.mockResolvedValue({
      catalog_id: "b436c224-5df5-4b4d-a772-1897f9147400",
      workflow_id: "c436c224-5df5-4b4d-a772-1897f9147400",
      catalog_token: "T".repeat(43),
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
      catalog_count: 1,
      observed_at: "2026-08-28T10:00:01Z",
      expires_at: "2026-08-28T10:10:01Z",
      companies: [
        {
          source_company_id: "sa-browser-v1-1234",
          display_name: "Hold My Beer OÜ",
        },
      ],
    });

    const originalPostMessage = window.postMessage.bind(window);
    vi.spyOn(window, "postMessage").mockImplementation(
      (message, targetOrigin) => {
        originalPostMessage(message, targetOrigin);
        if (!message || typeof message !== "object") return;
        const ping = message as Record<string, unknown>;
        if (
          ping.type !==
          "open-accounting.smartaccounts-browser-readiness-ping.v1"
        )
          return;
        window.dispatchEvent(
          new MessageEvent("message", {
            source: window,
            origin: window.location.origin,
            data: {
              source: "smartaccounts-browser-relay",
              type: "smartaccounts-browser-relay.readiness.v1",
              version: 1,
              nonce: ping.nonce,
              relay_protocol_version: "smartaccounts-browser-relay-v1",
              capture_manifest_version: "smartaccounts-brave-ui-v2",
              workflow_plan_version: "smartaccounts-browser-capture-plan-v1",
              smartaccounts_session_state: "signed_in",
              relay_build_version: "0.2.7",
            },
          }),
        );
      },
    );

    render(MigrationWorkbench, { tenantId: "" });
    await screen.findByText("Create or reuse companies from SmartAccounts");
    await screen.findByText("Relay ready — SmartAccounts is signed in.");
    expect(
      screen.queryByRole("button", { name: "Connect with Brave (no API key)" }),
    ).not.toBeInTheDocument();
    expect(apiMock.listMigrationExecutionRuns).not.toHaveBeenCalled();
    expect(apiMock.listMigrationProviderPresets).not.toHaveBeenCalled();
    expect(apiMock.getSmartAccountsSyncStatus).not.toHaveBeenCalled();
    expect(apiMock.getSmartAccountsBrowserCaptureStatus).not.toHaveBeenCalled();

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
          catalog_count: 1,
          catalog_sha256: "c".repeat(64),
        },
      }),
    );
    await screen.findByLabelText("All 1 relay-observed companies");
    expect(apiMock.getSmartAccountsSyncStatus).not.toHaveBeenCalled();
    expect(
      apiMock.getSmartAccountsBrowserCaptureWorkflowStatus,
    ).not.toHaveBeenCalled();
  });

  it("loads provider preset metadata into the bundle controls", async () => {
    render(MigrationWorkbench, { tenantId: "tenant-1" });

    await waitFor(() =>
      expect(apiMock.listMigrationProviderPresets).toHaveBeenCalledWith(
        "tenant-1",
      ),
    );
    await fireEvent.change(screen.getByLabelText("Provider preset"), {
      target: { value: "merit" },
    });

    expect(screen.getByText(/64 aliases/)).toBeInTheDocument();
    expect(screen.getByText("6 aliases")).toBeInTheDocument();
    expect(screen.getByText("konto -> code")).toBeInTheDocument();
    expect(screen.getByText("code, name, account_type")).toBeInTheDocument();

    await fireEvent.change(screen.getByLabelText("Provider preset"), {
      target: { value: "directo" },
    });

    expect(screen.getByText(/61 aliases/)).toBeInTheDocument();
    expect(screen.getByText("arve -> invoice_number")).toBeInTheDocument();
  });

  it("opens saved execution runs for monitoring", async () => {
    const monitoringRun = runningExecutionRun();
    apiMock.listMigrationExecutionRuns.mockResolvedValue([monitoringRun]);
    apiMock.getMigrationExecutionRun.mockResolvedValue(monitoringRun);
    render(MigrationWorkbench, { tenantId: "tenant-1" });

    expect(await screen.findByText("run-1")).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "Open" }));

    await waitFor(() =>
      expect(apiMock.getMigrationExecutionRun).toHaveBeenCalledWith(
        "tenant-1",
        "run-1",
      ),
    );
    expect(
      await screen.findByText("Saved migration run loaded."),
    ).toBeInTheDocument();
    expect(screen.getByText("Import running.")).toBeInTheDocument();
    expect(screen.getAllByText("50%").length).toBeGreaterThan(0);
    expect(screen.getAllByText("1.5s").length).toBeGreaterThan(0);
    expect(
      screen.getByText("Active step: #2 RUNNING contacts contacts-next.csv"),
    ).toBeInTheDocument();
  });

  it("filters saved execution runs by confirmation-ready status", async () => {
    render(MigrationWorkbench, { tenantId: "tenant-1" });

    await waitFor(() =>
      expect(apiMock.listMigrationExecutionRuns).toHaveBeenCalledWith(
        "tenant-1",
        {
          status: undefined,
          limit: 10,
        },
      ),
    );
    apiMock.listMigrationExecutionRuns.mockClear();

    await fireEvent.change(screen.getByLabelText("Status"), {
      target: { value: "needs_confirmation" },
    });
    await fireEvent.click(screen.getByRole("button", { name: "Refresh" }));

    await waitFor(() =>
      expect(apiMock.listMigrationExecutionRuns).toHaveBeenCalledWith(
        "tenant-1",
        {
          status: "needs_confirmation",
          limit: 10,
        },
      ),
    );
  });

  it("streams opened saved execution run telemetry into the dashboard", async () => {
    const monitoringRun = runningExecutionRun();
    const completedRun = completedStreamingRun();
    apiMock.listMigrationExecutionRuns.mockResolvedValue([monitoringRun]);
    apiMock.getMigrationExecutionRun.mockResolvedValue(monitoringRun);
    apiMock.watchMigrationExecutionRun.mockImplementation(
      async (
        _tenantId: string,
        _runId: string,
        options: {
          onEvent: (event: MigrationExecutionRunEvent) => void | Promise<void>;
        },
      ) => {
        await options.onEvent({
          type: "snapshot",
          sequence: 1,
          run: monitoringRun,
        });
        await options.onEvent({
          type: "complete",
          sequence: 2,
          run: completedRun,
        });
      },
    );
    render(MigrationWorkbench, { tenantId: "tenant-1" });

    expect(await screen.findByText("run-1")).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "Open" }));

    await waitFor(() =>
      expect(apiMock.watchMigrationExecutionRun).toHaveBeenCalledTimes(1),
    );
    expect(apiMock.watchMigrationExecutionRun).toHaveBeenCalledWith(
      "tenant-1",
      "run-1",
      expect.objectContaining({
        intervalMs: 1000,
        maxEvents: 1000,
        signal: expect.any(Object),
      }),
    );
    expect(
      await screen.findByText("Migration run stream completed."),
    ).toBeInTheDocument();
    expect(screen.getAllByText("100%").length).toBeGreaterThan(0);
    expect(screen.getByText("Imported contacts-next.csv.")).toBeInTheDocument();
  });

  it("opens a deep-linked saved execution run", async () => {
    const monitoringRun = runningExecutionRun();
    apiMock.listMigrationExecutionRuns.mockResolvedValue([monitoringRun]);
    apiMock.getMigrationExecutionRun.mockResolvedValue(monitoringRun);
    render(MigrationWorkbench, { tenantId: "tenant-1", runId: "run-1" });

    await waitFor(() =>
      expect(apiMock.getMigrationExecutionRun).toHaveBeenCalledWith(
        "tenant-1",
        "run-1",
      ),
    );
    expect(
      await screen.findByText("Saved migration run loaded."),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Active step: #2 RUNNING contacts contacts-next.csv"),
    ).toBeInTheDocument();
  });

  it("executes a confirmed cutover with a selected resume run id", async () => {
    apiMock.listMigrationExecutionRuns.mockResolvedValue([executionRun()]);
    render(MigrationWorkbench, { tenantId: "tenant-1" });

    expect(await screen.findByText("run-1")).toBeInTheDocument();
    await addContactsFile();
    await fireEvent.click(screen.getByRole("button", { name: "Resume" }));
    expect(screen.getByLabelText("Resume run ID")).toHaveValue("run-1");
    await fireEvent.click(screen.getByLabelText("Confirm execution"));
    await fireEvent.click(
      screen.getByRole("button", { name: "Execute confirmed cutover" }),
    );

    await waitFor(() =>
      expect(apiMock.executeMigration).toHaveBeenCalledTimes(1),
    );
    expect(apiMock.executeMigration).toHaveBeenCalledWith(
      "tenant-1",
      expect.objectContaining({
        confirm: true,
        resume_from_run_id: "run-1",
        files: [
          {
            kind: "contacts",
            file_name: "contacts.csv",
            csv_content: "name\nAcme OU\n",
          },
        ],
      }),
    );
    expect(
      await screen.findByText("Migration execution run completed."),
    ).toBeInTheDocument();
  });

  it("resumes a confirmed saved cutover without requiring a fresh bundle upload", async () => {
    apiMock.listMigrationExecutionRuns.mockResolvedValue([executionRun()]);
    render(MigrationWorkbench, { tenantId: "tenant-1" });

    expect(await screen.findByText("run-1")).toBeInTheDocument();
    await fireEvent.click(screen.getByRole("button", { name: "Resume" }));
    expect(screen.getByLabelText("Resume run ID")).toHaveValue("run-1");
    await fireEvent.click(screen.getByLabelText("Confirm execution"));
    await fireEvent.click(
      screen.getByRole("button", { name: "Execute confirmed cutover" }),
    );

    await waitFor(() =>
      expect(apiMock.executeMigration).toHaveBeenCalledTimes(1),
    );
    expect(apiMock.executeMigration).toHaveBeenCalledWith(
      "tenant-1",
      expect.objectContaining({
        confirm: true,
        resume_from_run_id: "run-1",
        files: [],
      }),
    );
  });
});
