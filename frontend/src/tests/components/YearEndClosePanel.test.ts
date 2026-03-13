import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { cleanup, fireEvent, render, screen } from "@testing-library/svelte";
import Decimal from "decimal.js";
import { baseLocale, setLocale } from "$lib/paraglide/runtime.js";
import YearEndClosePanel from "$lib/components/YearEndClosePanel.svelte";
import type { YearEndCloseStatus } from "$lib/api";

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
    ...overrides,
  };
}

describe("YearEndClosePanel", () => {
  afterEach(() => {
    cleanup();
  });

  beforeEach(() => {
    setLocale(baseLocale, { reload: false });
  });

  it("shows a ready carry-forward state and triggers actions", async () => {
    const onrefresh = vi.fn();
    const onsubmit = vi.fn();
    const onperiodenddatechange = vi.fn();

    render(YearEndClosePanel, {
      status: createStatus(),
      periodEndDate: "2025-12-31",
      onrefresh,
      onsubmit,
      onperiodenddatechange,
    });

    expect(screen.getByText("Fiscal-year close")).toBeInTheDocument();
    expect(screen.getByText("Ready")).toBeInTheDocument();
    expect(screen.getByText("2025")).toBeInTheDocument();
    expect(screen.getByText("€600.00")).toBeInTheDocument();

    await fireEvent.click(
      screen.getByRole("button", { name: "Refresh status" }),
    );
    expect(onrefresh).toHaveBeenCalledTimes(1);

    await fireEvent.click(
      screen.getByRole("button", { name: "Run carry-forward" }),
    );
    expect(onsubmit).toHaveBeenCalledTimes(1);

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
});
