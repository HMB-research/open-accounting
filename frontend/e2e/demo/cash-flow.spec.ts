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

interface CashFlowStatementResponse {
  start_date: string;
  end_date: string;
}

function isCashFlowStatementResponse(response: Response): boolean {
  return (
    response.request().method() === "GET" &&
    response.status() === 200 &&
    /\/api\/v1\/tenants\/[^/]+\/reports\/cash-flow$/.test(
      new URL(response.url()).pathname,
    )
  );
}

async function openCashFlowReport(
  page: Page,
  testInfo: TestInfo,
): Promise<Response> {
  const reportResponsePromise = page.waitForResponse(
    isCashFlowStatementResponse,
  );
  await navigateTo(page, "/reports/cash-flow", testInfo, {
    waitForNetworkIdle: false,
  });
  const reportResponse = await reportResponsePromise;
  await waitForRouteReady(
    page,
    ".controls-section, .report-container, .alert-error, .empty-state",
  );
  await expect(
    page.getByRole("heading", { name: /cash flow|rahavoog/i }).first(),
  ).toBeVisible();
  await expect(
    page.getByRole("button", { name: /generate|genereeri/i }),
  ).toBeEnabled({
    timeout: 15000,
  });
  return reportResponse;
}

test.describe("Demo Cash Flow Statement", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
  });

  test("renders report controls, statement sections, and regenerates a changed period", async ({
    page,
  }, testInfo) => {
    await openCashFlowReport(page, testInfo);
    const startDate = page.locator("input#startDate");
    const endDate = page.locator("input#endDate");
    const generateButton = page.getByRole("button", {
      name: /generate|genereeri/i,
    });
    const report = page.locator(".report-container");

    await expect(startDate).toBeVisible();
    await expect(endDate).toBeVisible();
    await expect(generateButton).toBeVisible();
    await expect(generateButton).toBeEnabled();
    await expect(
      page.getByRole("link", { name: /back|tagasi/i }),
    ).toBeVisible();

    await expect(report).toBeVisible({ timeout: 10000 });
    await expect(
      report.getByRole("heading", { name: /cash flow|rahavoog/i }),
    ).toBeVisible();
    await expect(
      report.getByRole("heading", { name: /operating|äritegevus/i }),
    ).toBeVisible();
    await expect(
      report.getByRole("heading", { name: /investing|investeerimis/i }),
    ).toBeVisible();
    await expect(
      report.getByRole("heading", { name: /financing|finantseerimis/i }),
    ).toBeVisible();
    await expect(report).toContainText(
      /net change|opening cash|closing cash|raha muutus|raha perioodi/i,
    );

    await page.locator("input#startDate").fill("2026-01-01");
    await page.locator("input#endDate").fill("2026-12-31");

    const regeneratedReportPromise = page.waitForResponse((response) => {
      if (!isCashFlowStatementResponse(response)) return false;
      const url = new URL(response.url());
      return (
        url.searchParams.get("start_date") === "2026-01-01" &&
        url.searchParams.get("end_date") === "2026-12-31"
      );
    });
    await generateButton.click();
    const regeneratedReport = await regeneratedReportPromise;
    const body = (await regeneratedReport.json()) as CashFlowStatementResponse;

    expect(body.start_date).toBe("2026-01-01");
    expect(body.end_date).toBe("2026-12-31");
    await expect(page.locator(".report-container")).toBeVisible({
      timeout: 10000,
    });
    await expect(page.locator(".period")).toContainText("2026-01-01");
    await expect(page.locator(".period")).toContainText("2026-12-31");
  });
});
