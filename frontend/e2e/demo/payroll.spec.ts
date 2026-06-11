import {
  test,
  expect,
  type Page,
  type Response,
  type TestInfo,
} from "@playwright/test";
import {
  ensureAuthenticated,
  navigateTo,
  ensureDemoTenant,
  waitForRouteReady,
} from "./utils";

interface PayrollRunResponse {
  id: string;
  period_year: number;
  period_month: number;
  status: string;
  total_gross: string | number;
  total_net: string | number;
  total_employer_cost: string | number;
}

interface PayslipResponse {
  gross_salary: string | number;
  net_salary: string | number;
  total_employer_cost: string | number;
}

function responsePath(responseUrl: string): string {
  return new URL(responseUrl).pathname;
}

function payrollRunsResponseForYear(response: Response, year: number): boolean {
  const url = new URL(response.url());

  return (
    response.request().method() === "GET" &&
    response.status() === 200 &&
    responsePath(response.url()).endsWith("/payroll-runs") &&
    url.searchParams.get("year") === String(year)
  );
}

function formatAmount(value: string | number): string {
  return Number(value).toFixed(2);
}

async function openPayrollPage(page: Page, testInfo: TestInfo): Promise<void> {
  const currentYear = new Date().getFullYear();
  const initialPayrollResponse = page.waitForResponse((response) =>
    payrollRunsResponseForYear(response, currentYear),
  );

  await navigateTo(page, "/payroll", testInfo, { waitForNetworkIdle: false });
  await waitForRouteReady(
    page,
    "main h1, .empty-state, .table-container, .info-box",
  );
  await initialPayrollResponse;
}

test.describe("Demo Payroll", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
  });

  test("verifies seeded payroll run filtering and payslips workflow", async ({
    page,
  }, testInfo) => {
    await openPayrollPage(page, testInfo);

    await expect(
      page.getByRole("heading", { name: /payroll|palgaarvestus/i, level: 1 }),
    ).toBeVisible();
    await expect(
      page
        .locator(".header-actions")
        .getByRole("button", { name: /new payroll run|uus palgaarvestus/i }),
    ).toBeVisible();
    await expect(
      page
        .locator(".header-actions")
        .getByRole("button", { name: /import history|impordi ajalugu/i }),
    ).toBeVisible();

    await page
      .locator(".header-actions")
      .getByRole("button", { name: /new payroll run|uus palgaarvestus/i })
      .click();
    const createDialog = page.getByRole("dialog", {
      name: /create payroll run|loo palgaarvestus/i,
    });
    await expect(createDialog).toBeVisible();
    await expect(createDialog.locator("select#year")).toBeVisible();
    await expect(createDialog.locator("select#month")).toBeVisible();
    await createDialog.getByRole("button", { name: /cancel|tühista/i }).click();
    await expect(createDialog).toBeHidden();

    await page
      .locator(".header-actions")
      .getByRole("button", { name: /import history|impordi ajalugu/i })
      .click();
    const importDialog = page.getByRole("dialog", {
      name: /import history|impordi ajalugu/i,
    });
    await expect(importDialog).toBeVisible();
    await expect(
      importDialog.locator("input#payroll-history-file"),
    ).toBeVisible();
    await expect(
      importDialog.getByRole("button", {
        name: /download template|laadi mall alla/i,
      }),
    ).toBeVisible();
    await importDialog.getByRole("button", { name: /cancel|tühista/i }).click();
    await expect(importDialog).toBeHidden();

    const yearDropdown = page.locator("main select#yearFilter");
    await expect(yearDropdown).toBeVisible({ timeout: 10000 });
    await expect(yearDropdown.locator('option[value="2024"]')).toBeAttached();

    const payrollRunsResponse = page.waitForResponse((response) =>
      payrollRunsResponseForYear(response, 2024),
    );
    await yearDropdown.selectOption("2024");
    const payrollRuns = (await (
      await payrollRunsResponse
    ).json()) as PayrollRunResponse[];
    expect(payrollRuns.length).toBeGreaterThan(0);

    const rows = page.locator("table tbody tr");
    await expect(rows).toHaveCount(payrollRuns.length);

    const firstRun = payrollRuns[0];
    const firstRunRow = rows.first();
    await expect(firstRunRow).toContainText(formatAmount(firstRun.total_gross));
    await expect(firstRunRow).toContainText(formatAmount(firstRun.total_net));
    await expect(firstRunRow).toContainText(
      formatAmount(firstRun.total_employer_cost),
    );

    const taxRates = page.locator(".info-box");
    await expect(
      taxRates.getByRole("heading", { name: /tax rates|maksude määrad/i }),
    ).toBeVisible();
    await expect(taxRates).toContainText("22%");
    await expect(taxRates).toContainText("33%");

    const payslipsResponse = page.waitForResponse((response) => {
      return (
        response.request().method() === "GET" &&
        response.status() === 200 &&
        responsePath(response.url()).endsWith(
          `/payroll-runs/${firstRun.id}/payslips`,
        )
      );
    });
    await firstRunRow
      .getByRole("button", { name: /payslips|palgalehed/i })
      .click();
    const payslips = (await (
      await payslipsResponse
    ).json()) as PayslipResponse[];
    expect(payslips.length).toBeGreaterThan(0);

    const payslipsDialog = page.getByRole("dialog", {
      name: /payslips|palgalehed/i,
    });
    await expect(payslipsDialog).toBeVisible();
    await expect(payslipsDialog.locator("tbody tr")).toHaveCount(
      payslips.length,
    );
    await expect(payslipsDialog).toContainText(
      formatAmount(payslips[0].gross_salary),
    );
    await expect(payslipsDialog).toContainText(
      formatAmount(payslips[0].net_salary),
    );
    await expect(payslipsDialog).toContainText(
      formatAmount(payslips[0].total_employer_cost),
    );

    await payslipsDialog.getByRole("button", { name: /close|sulge/i }).click();
    await expect(payslipsDialog).toBeHidden();
  });
});
