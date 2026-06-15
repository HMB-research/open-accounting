import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen, waitFor } from "@testing-library/svelte";
import Decimal from "decimal.js";
import { baseLocale, setLocale } from "$lib/paraglide/runtime.js";
import YearEndClosePanel from "$lib/components/YearEndClosePanel.svelte";
import type { YearEndCloseStatus } from "$lib/api";

const apiMock = vi.hoisted(() => ({
  downloadYearEndCloseAuditArchive: vi.fn(),
  reverseYearEndCarryForward: vi.fn(),
}));

vi.mock("$lib/api", async (importOriginal) => {
  const actual = await importOriginal<typeof import("$lib/api")>();
  return {
    ...actual,
    api: {
      ...actual.api,
      downloadYearEndCloseAuditArchive: apiMock.downloadYearEndCloseAuditArchive,
      reverseYearEndCarryForward: apiMock.reverseYearEndCarryForward,
    },
  };
});

function createStatus(
  overrides: Partial<YearEndCloseStatus> = {},
): YearEndCloseStatus {
  return {
    period_end_date: "2025-12-31",
    fiscal_year_label: "2025",
    fiscal_year_start_date: "2025-01-01",
    fiscal_year_end_date: "2025-12-31",
    carry_forward_date: "2026-01-01",
    locked_through_date: "2025-12-31",
    is_fiscal_year_end: true,
    period_closed: true,
    has_profit_and_loss_activity: true,
    carry_forward_needed: true,
    carry_forward_ready: true,
    has_retained_earnings_account: true,
    retained_earnings_account: {
      id: "retained",
      code: "3200",
      name: "Retained Earnings",
    },
    net_income: new Decimal(600),
    existing_carry_forward: null,
    close_pack_evidence_entity_id: "39b4b67a-4d8f-4a43-9fdf-59bfc6bb4f1b",
    close_pack_evidence: {
      entity_type: "year_end_close",
      entity_id: "39b4b67a-4d8f-4a43-9fdf-59bfc6bb4f1b",
      compliant: true,
      total_count: 1,
      pending_review_count: 0,
      reviewed_count: 0,
      approved_count: 1,
      rejected_count: 0,
      missing_evidence: false,
      document_type_counts: { close_pack: 1 },
      approved_document_type_counts: { close_pack: 1 },
      rule_results: [],
      violations: [],
    },
    ...overrides,
  };
}

describe("YearEndClosePanel", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    setLocale(baseLocale, { reload: false });
    apiMock.downloadYearEndCloseAuditArchive.mockReset();
    apiMock.downloadYearEndCloseAuditArchive.mockResolvedValue(undefined);
    apiMock.reverseYearEndCarryForward.mockReset();
    apiMock.reverseYearEndCarryForward.mockResolvedValue({
      reversal_journal_entry: {
        id: "je-reversal",
        entry_number: "JE-00101",
      },
      status: { period_end_date: "2025-12-31" },
    });
  });

  it("shows a ready carry-forward state and triggers actions", async () => {
    const onrefresh = vi.fn();
    const onsubmit = vi.fn();
    const onperiodenddatechange = vi.fn();

    render(YearEndClosePanel, {
      status: createStatus(),
      tenantId: "tenant-1",
      periodEndDate: "2025-12-31",
      onrefresh,
      onsubmit,
      onperiodenddatechange,
    });

    expect(screen.getByText("Fiscal-year close")).toBeInTheDocument();
    expect(screen.getByText("Ready")).toBeInTheDocument();
    expect(screen.getByText("2025")).toBeInTheDocument();
    expect(screen.getByText("€600.00")).toBeInTheDocument();
    expect(screen.getByText("Required approval pack")).toBeInTheDocument();
    expect(
      screen.getAllByText("1 approved close-pack document is attached."),
    ).toHaveLength(2);

    await fireEvent.click(
      screen.getByRole("button", { name: "Refresh status" }),
    );
    expect(onrefresh).toHaveBeenCalledTimes(1);

    await fireEvent.click(
      screen.getByRole("button", { name: "Run carry-forward" }),
    );
    expect(onsubmit).toHaveBeenCalledTimes(1);

    await fireEvent.click(
      screen.getByRole("button", { name: "Download audit archive" }),
    );
    expect(apiMock.downloadYearEndCloseAuditArchive).toHaveBeenCalledWith(
      "tenant-1",
      "2025-12-31",
    );

    await fireEvent.change(screen.getByLabelText("Target year-end date"), {
      target: { value: "2024-12-31" },
    });
    expect(onperiodenddatechange).toHaveBeenCalledWith("2024-12-31");
  });

  it("shows the completed state once a carry-forward entry already exists", () => {
    render(YearEndClosePanel, {
      status: createStatus({
        carry_forward_needed: false,
        carry_forward_ready: false,
        existing_carry_forward: {
          id: "je-1",
          entry_number: "JE-00100",
          entry_date: "2026-01-01",
          description: "Year-end carry-forward",
          status: "POSTED",
        },
      }),
      periodEndDate: "2025-12-31",
    });

    expect(screen.getByText("Completed")).toBeInTheDocument();
    expect(screen.getByText(/JE-00100/)).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Run carry-forward" }),
    ).toBeDisabled();
  });

  it("reverses an already-posted carry-forward with a reason", async () => {
    const onrefresh = vi.fn();

    render(YearEndClosePanel, {
      status: createStatus({
        carry_forward_needed: false,
        carry_forward_ready: false,
        existing_carry_forward: {
          id: "je-1",
          entry_number: "JE-00100",
          entry_date: "2026-01-01",
          description: "Year-end carry-forward",
          status: "POSTED",
        },
      }),
      tenantId: "tenant-1",
      periodEndDate: "2025-12-31",
      onrefresh,
    });

    const reverseButton = screen.getByRole("button", {
      name: "Reverse carry-forward",
    });
    expect(reverseButton).toBeDisabled();

    await fireEvent.input(screen.getByLabelText("Reversal reason"), {
      target: { value: "Late supplier accrual" },
    });

    await waitFor(() => {
      expect(reverseButton).not.toBeDisabled();
    });

    await fireEvent.click(reverseButton);

    await waitFor(() => {
      expect(apiMock.reverseYearEndCarryForward).toHaveBeenCalledWith(
        "tenant-1",
        {
          period_end_date: "2025-12-31",
          reason: "Late supplier accrual",
        },
      );
      expect(onrefresh).toHaveBeenCalledTimes(1);
      expect(
        screen.getByText("Carry-forward reversed in JE-00101."),
      ).toBeInTheDocument();
    });
  });

  it("blocks carry-forward when close-pack evidence still needs approval", () => {
    render(YearEndClosePanel, {
      status: createStatus({
        close_pack_evidence: {
          entity_type: "year_end_close",
          entity_id: "39b4b67a-4d8f-4a43-9fdf-59bfc6bb4f1b",
          compliant: false,
          total_count: 1,
          pending_review_count: 1,
          reviewed_count: 0,
          approved_count: 0,
          rejected_count: 0,
          missing_evidence: false,
          document_type_counts: { close_pack: 1 },
          approved_document_type_counts: { close_pack: 0 },
          rule_results: [],
          violations: [],
        },
      }),
      tenantId: "tenant-1",
      periodEndDate: "2025-12-31",
    });

    expect(screen.getByText("Needs review")).toBeInTheDocument();
    expect(screen.getByText("Approve close-pack evidence")).toBeInTheDocument();
    expect(
      screen.getByRole("button", { name: "Run carry-forward" }),
    ).toBeDisabled();
  });
});
