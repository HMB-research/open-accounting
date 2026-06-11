import { test, expect } from "@playwright/test";
import {
  ensureAuthenticated,
  navigateTo,
  ensureDemoTenant,
  waitForRouteReady,
} from "./utils";

function responsePath(responseUrl: string): string {
  return new URL(responseUrl).pathname;
}

async function openSalaryCalculator(
  page: import("@playwright/test").Page,
  testInfo: import("@playwright/test").TestInfo,
) {
  const initialPreviewResponse = page.waitForResponse(
    (response) =>
      response.request().method() === "POST" &&
      responsePath(response.url()).endsWith("/payroll/tax-preview") &&
      response.status() === 200,
  );
  await navigateTo(page, "/payroll/calculator", testInfo, {
    waitForNetworkIdle: false,
  });
  await waitForRouteReady(page, "main h1, .container h1");
  await initialPreviewResponse;
}

test.describe("Demo Salary Calculator", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
  });

  test("calculates salary preview from route controls", async ({
    page,
  }, testInfo) => {
    await openSalaryCalculator(page, testInfo);

    await expect(
      page.getByRole("heading", {
        name: /Salary Calculator|Palgakalkulaator/i,
      }),
    ).toBeVisible();

    const grossSalaryInput = page.locator("input#grossSalary");
    const basicExemptionInput = page.locator("input#basicExemption");
    const pensionRateSelect = page.locator("select#pensionRate");
    const applyBasicExemptionCheckbox = page.locator(
      'label.checkbox-label input[type="checkbox"]',
    );
    const resultsSection = page.locator(".results-section");

    await expect(grossSalaryInput).toBeVisible();
    await expect(basicExemptionInput).toBeVisible();
    await expect(pensionRateSelect).toBeVisible();
    await expect(applyBasicExemptionCheckbox).toBeChecked();
    await expect(
      resultsSection.getByRole("heading", { name: /Results|Tulemused/i }),
    ).toBeVisible();

    await grossSalaryInput.fill("3000");
    await basicExemptionInput.fill("500");
    const recalculationResponse = page.waitForResponse(
      (response) =>
        response.request().method() === "POST" &&
        responsePath(response.url()).endsWith("/payroll/tax-preview") &&
        response.status() === 200,
    );
    await pensionRateSelect.selectOption("0.04");

    const response = await recalculationResponse;
    const responseBody = await response.json();
    expect(responseBody).toEqual(
      expect.objectContaining({
        gross_salary: 3000,
        basic_exemption: 500,
        taxable_income: 2500,
        income_tax: 550,
        funded_pension: 120,
        net_salary: 2282,
        total_employer_cost: 4014,
      }),
    );

    await expect(grossSalaryInput).toHaveValue("3000");
    await expect(basicExemptionInput).toHaveValue("500");
    await expect(pensionRateSelect).toHaveValue("0.04");
    await expect(
      resultsSection.getByText("3000.00 EUR", { exact: true }).first(),
    ).toBeVisible();
    await expect(
      resultsSection.getByText("500.00 EUR", { exact: true }),
    ).toBeVisible();
    await expect(
      resultsSection.getByText("2282.00 EUR", { exact: true }),
    ).toBeVisible();
  });
});
