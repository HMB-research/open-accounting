import { test, expect, type Page, type Response } from "@playwright/test";
import {
  setupCostCentersPage,
  waitForCostCentersPageReady,
} from "./cost-centers-utils";

interface CostCenterResponse {
  id: string;
  code: string;
  name: string;
  description?: string;
  is_active: boolean;
  budget_amount?: string;
  budget_period?: string;
}

function isCostCenterCreateResponse(response: Response): boolean {
  return (
    response.request().method() === "POST" &&
    /\/api\/v1\/tenants\/[^/]+\/cost-centers$/.test(
      new URL(response.url()).pathname,
    )
  );
}

function isCostCenterUpdateResponse(costCenterId: string) {
  return (response: Response): boolean =>
    response.request().method() === "PUT" &&
    new URL(response.url()).pathname.endsWith(`/cost-centers/${costCenterId}`);
}

function isCostCenterDeleteResponse(costCenterId: string) {
  return (response: Response): boolean =>
    response.request().method() === "DELETE" &&
    new URL(response.url()).pathname.endsWith(`/cost-centers/${costCenterId}`);
}

function costCenterRows(page: Page) {
  return page.locator("table.table").first().locator("tbody tr");
}

test.describe("Demo Cost Centers - CRUD Workflow", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await setupCostCentersPage(page, testInfo);
  });

  test("creates, edits, and deletes a cost center", async ({
    page,
  }, testInfo) => {
    const suffix =
      `${testInfo.parallelIndex}${testInfo.retry}${Date.now().toString(36)}`.slice(
        -10,
      );
    const code = `E2E${suffix}`.slice(0, 20);
    const updatedCode = `U${code}`.slice(0, 20);
    const name = `E2E Cost Center ${suffix}`;
    const updatedName = `E2E Cost Center Updated ${suffix}`;

    await page
      .getByRole("button", {
        name: /add cost center|lisa kulukoht|add|lisa|\+/i,
      })
      .first()
      .click();
    const createModal = page
      .locator(".modal.show, .modal")
      .filter({ has: page.locator("#code") })
      .first();
    await expect(createModal).toBeVisible({ timeout: 5000 });

    await createModal.locator("#code").fill(code);
    await createModal.locator("#name").fill(name);
    await createModal
      .locator("#description")
      .fill("Created by demo E2E cost-center workflow");
    await createModal.locator("#budgetAmount").fill("2500.00");
    await createModal.locator("#budgetPeriod").selectOption("MONTHLY");

    const createResponsePromise = page.waitForResponse(
      isCostCenterCreateResponse,
    );
    await createModal.getByRole("button", { name: /^save$/i }).click();
    const createResponse = await createResponsePromise;
    expect(createResponse.status()).toBe(201);
    const createdCostCenter =
      (await createResponse.json()) as CostCenterResponse;

    expect(createdCostCenter.id).toBeTruthy();
    expect(createdCostCenter.code).toBe(code);
    expect(createdCostCenter.name).toBe(name);
    expect(createdCostCenter.description).toBe(
      "Created by demo E2E cost-center workflow",
    );
    expect(createdCostCenter.is_active).toBe(true);
    expect(createdCostCenter.budget_period).toBe("MONTHLY");

    await waitForCostCentersPageReady(page);
    await expect(createModal).toBeHidden();

    const createdRow = costCenterRows(page).filter({ hasText: code });
    await expect(createdRow).toBeVisible();
    await expect(createdRow).toContainText(name);
    await expect(createdRow).toContainText(/active|aktiivne/i);

    await createdRow.getByRole("button", { name: /edit|muuda/i }).click();
    const editModal = page
      .locator(".modal.show, .modal")
      .filter({ has: page.locator("#code") })
      .first();
    await expect(editModal).toBeVisible({ timeout: 5000 });
    await expect(editModal.locator("#code")).toHaveValue(code);

    await editModal.locator("#code").fill(updatedCode);
    await editModal.locator("#name").fill(updatedName);
    await editModal
      .locator("#description")
      .fill("Updated by demo E2E cost-center workflow");
    await editModal.locator("#budgetAmount").fill("3750.00");
    await editModal.locator("#budgetPeriod").selectOption("QUARTERLY");

    const updateResponsePromise = page.waitForResponse(
      isCostCenterUpdateResponse(createdCostCenter.id),
    );
    await editModal.getByRole("button", { name: /^save$/i }).click();
    const updateResponse = await updateResponsePromise;
    expect(updateResponse.status()).toBe(200);
    const updatedCostCenter =
      (await updateResponse.json()) as CostCenterResponse;

    expect(updatedCostCenter.id).toBe(createdCostCenter.id);
    expect(updatedCostCenter.code).toBe(updatedCode);
    expect(updatedCostCenter.name).toBe(updatedName);
    expect(updatedCostCenter.description).toBe(
      "Updated by demo E2E cost-center workflow",
    );
    expect(updatedCostCenter.budget_period).toBe("QUARTERLY");

    await waitForCostCentersPageReady(page);
    await expect(editModal).toBeHidden();

    const updatedRow = costCenterRows(page).filter({ hasText: updatedCode });
    await expect(updatedRow).toBeVisible();
    await expect(updatedRow).toContainText(updatedName);
    await expect(page.getByText(code, { exact: true })).toHaveCount(0);

    await updatedRow.getByRole("button", { name: /delete|kustuta/i }).click();
    const deleteModal = page
      .locator(".modal.show, .modal")
      .filter({ hasText: /confirm delete|kinnita kustutamine/i })
      .first();
    await expect(deleteModal).toBeVisible({ timeout: 5000 });
    await expect(deleteModal).toContainText(updatedName);

    const deleteResponsePromise = page.waitForResponse(
      isCostCenterDeleteResponse(createdCostCenter.id),
    );
    await deleteModal.getByRole("button", { name: /delete|kustuta/i }).click();
    const deleteResponse = await deleteResponsePromise;
    expect(deleteResponse.status()).toBe(204);

    await waitForCostCentersPageReady(page);
    await expect(
      costCenterRows(page).filter({ hasText: updatedCode }),
    ).toHaveCount(0);
  });
});
