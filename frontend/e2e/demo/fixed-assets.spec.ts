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

interface FixedAssetResponse {
  id: string;
  asset_number: string;
  name: string;
  status: string;
  category_id?: string;
  serial_number?: string;
  location?: string;
  depreciation_method?: string;
  useful_life_months?: number;
  residual_value?: string;
}

function isAssetsListResponse(response: Response): boolean {
  return (
    response.request().method() === "GET" &&
    response.status() === 200 &&
    /\/api\/v1\/tenants\/[^/]+\/assets$/.test(new URL(response.url()).pathname)
  );
}

function isAssetCreateResponse(response: Response): boolean {
  return (
    response.request().method() === "POST" &&
    /\/api\/v1\/tenants\/[^/]+\/assets$/.test(new URL(response.url()).pathname)
  );
}

function isAssetUpdateResponse(assetId: string) {
  return (response: Response): boolean =>
    response.request().method() === "PUT" &&
    new URL(response.url()).pathname.endsWith(`/assets/${assetId}`);
}

function isAssetDeleteResponse(assetId: string) {
  return (response: Response): boolean =>
    response.request().method() === "DELETE" &&
    new URL(response.url()).pathname.endsWith(`/assets/${assetId}`);
}

async function waitForAssetsReady(page: Page) {
  await waitForRouteReady(page, ".filters, table, .empty-state, .alert-error");
  await page
    .getByText(/^Loading\.\.\.$/)
    .waitFor({ state: "hidden", timeout: 10000 })
    .catch(() => {});
  await expect(
    page.getByRole("heading", { name: /fixed assets|assets/i }),
  ).toBeVisible();
  await expect(
    page.locator("table, .empty-state, .alert-error").first(),
  ).toBeVisible();
}

async function openAssets(
  page: Page,
  testInfo: TestInfo,
): Promise<FixedAssetResponse[]> {
  const assetsResponsePromise = page.waitForResponse(isAssetsListResponse);
  await navigateTo(page, "/assets", testInfo, { waitForNetworkIdle: false });
  const assetsResponse = await assetsResponsePromise;
  await waitForAssetsReady(page);
  return (await assetsResponse.json()) as FixedAssetResponse[];
}

function assetRows(page: Page) {
  return page.locator("table tbody tr");
}

function statusFilter(page: Page) {
  return page.locator(".filters select").first();
}

test.describe("Fixed Assets View", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
  });

  test("renders seeded asset details and filters by status", async ({
    page,
  }, testInfo) => {
    const assets = await openAssets(page, testInfo);
    const rows = assetRows(page);

    expect(assets.length).toBeGreaterThanOrEqual(6);
    await expect(rows).toHaveCount(assets.length);
    await expect(page.locator(".filters select").first()).toBeVisible();
    await expect(
      page.getByRole("button", { name: /new asset|new|create|add/i }),
    ).toBeVisible();
    await expect(page.locator("table thead")).toContainText(/category/i);

    const serverRow = rows.filter({ hasText: "Dell PowerEdge Server" });
    await expect(serverRow).toBeVisible();
    await expect(serverRow).toContainText("FA-2024-001");
    await expect(serverRow).toContainText(/active/i);
    await expect(serverRow).toContainText("IT Equipment");
    await expect(serverRow.locator("td").nth(0)).toBeVisible();
    await expect(serverRow.locator("td").nth(1)).toBeVisible();

    await expect(rows.filter({ hasText: "Old Projector" })).toContainText(
      /disposed/i,
    );
    await expect(rows.filter({ hasText: "New Monitor Setup" })).toContainText(
      /draft/i,
    );

    const draftAssetsResponsePromise = page.waitForResponse((response) => {
      if (!isAssetsListResponse(response)) return false;
      return new URL(response.url()).searchParams.get("status") === "DRAFT";
    });
    await statusFilter(page).selectOption("DRAFT");
    const draftAssetsResponse = await draftAssetsResponsePromise;
    const draftAssets =
      (await draftAssetsResponse.json()) as FixedAssetResponse[];

    expect(draftAssets).toHaveLength(1);
    expect(draftAssets[0]?.name).toBe("New Monitor Setup");
    expect(draftAssets[0]?.status).toBe("DRAFT");
    await waitForAssetsReady(page);
    await expect(statusFilter(page)).toHaveValue("DRAFT");
    await expect(assetRows(page)).toHaveCount(draftAssets.length);
    await expect(assetRows(page).first()).toContainText("New Monitor Setup");
    await expect(assetRows(page).first()).toContainText(/draft/i);
    await expect(page.getByText("Old Projector")).toHaveCount(0);
  });

  test("creates, edits, and deletes a draft asset", async ({
    page,
  }, testInfo) => {
    await openAssets(page, testInfo);

    const suffix = `${testInfo.parallelIndex}-${testInfo.retry}-${Date.now().toString(36)}`;
    const assetName = `E2E Asset ${suffix}`;
    const updatedAssetName = `E2E Asset Updated ${suffix}`;
    const serialNumber = `SN-${suffix}`;
    const location = `Shelf ${suffix}`;
    const updatedSerialNumber = `SN-UPD-${suffix}`;
    const updatedLocation = `Storage ${suffix}`;

    await page.getByRole("button", { name: /new asset/i }).click();
    const createDialog = page.getByRole("dialog", { name: /new asset/i });
    await expect(createDialog).toBeVisible();

    await createDialog.locator("#name").fill(assetName);
    await createDialog
      .locator("#category")
      .selectOption({ label: "IT Equipment" });
    await createDialog
      .locator("#description")
      .fill("Created by demo E2E fixed-assets workflow");
    await createDialog.locator("#purchase-date").fill("2026-01-15");
    await createDialog.locator("#purchase-cost").fill("1250.00");
    await createDialog.locator("#serial-number").fill(serialNumber);
    await createDialog.locator("#location").fill(location);
    await createDialog
      .locator("#depreciation-method")
      .selectOption("STRAIGHT_LINE");
    await createDialog.locator("#useful-life").fill("48");
    await createDialog.locator("#residual-value").fill("125.00");
    await createDialog.locator("#depreciation-start").fill("2026-02-01");

    const createResponsePromise = page.waitForResponse(isAssetCreateResponse);
    await createDialog.getByRole("button", { name: /create asset/i }).click();
    const createResponse = await createResponsePromise;
    expect(createResponse.status()).toBe(201);
    const createdAsset = (await createResponse.json()) as FixedAssetResponse;

    expect(createdAsset.id).toBeTruthy();
    expect(createdAsset.asset_number).toBeTruthy();
    expect(createdAsset.name).toBe(assetName);
    expect(createdAsset.status).toBe("DRAFT");
    expect(createdAsset.serial_number).toBe(serialNumber);
    expect(createdAsset.location).toBe(location);
    await expect(createDialog).toBeHidden();

    const createdRow = assetRows(page).filter({ hasText: assetName });
    await expect(createdRow).toBeVisible();
    await expect(createdRow).toContainText(/draft/i);
    await expect(createdRow).toContainText("IT Equipment");

    await createdRow.getByRole("button", { name: /edit/i }).click();
    const editDialog = page.getByRole("dialog", {
      name: new RegExp(`edit:\\s*${createdAsset.asset_number}`, "i"),
    });
    await expect(editDialog).toBeVisible();

    await expect(editDialog.locator("#edit-name")).toHaveValue(assetName);
    await editDialog.locator("#edit-name").fill(updatedAssetName);
    await editDialog
      .locator("#edit-description")
      .fill("Updated by demo E2E fixed-assets workflow");
    await editDialog.locator("#edit-serial-number").fill(updatedSerialNumber);
    await editDialog.locator("#edit-location").fill(updatedLocation);
    await editDialog
      .locator("#edit-depreciation-method")
      .selectOption("DECLINING_BALANCE");
    await editDialog.locator("#edit-useful-life").fill("36");
    await editDialog.locator("#edit-residual-value").fill("250.00");

    const updateResponsePromise = page.waitForResponse(
      isAssetUpdateResponse(createdAsset.id),
    );
    await editDialog.getByRole("button", { name: /^save$/i }).click();
    const updateResponse = await updateResponsePromise;
    expect(updateResponse.status()).toBe(200);
    const updatedAsset = (await updateResponse.json()) as FixedAssetResponse;

    expect(updatedAsset.id).toBe(createdAsset.id);
    expect(updatedAsset.name).toBe(updatedAssetName);
    expect(updatedAsset.status).toBe("DRAFT");
    expect(updatedAsset.serial_number).toBe(updatedSerialNumber);
    expect(updatedAsset.location).toBe(updatedLocation);
    expect(updatedAsset.depreciation_method).toBe("DECLINING_BALANCE");
    expect(updatedAsset.useful_life_months).toBe(36);
    await expect(editDialog).toBeHidden();

    const updatedRow = assetRows(page).filter({ hasText: updatedAssetName });
    await expect(updatedRow).toBeVisible();
    await expect(updatedRow).toContainText(/draft/i);
    await expect(page.getByText(assetName)).toHaveCount(0);

    page.once("dialog", async (confirmDialog) => {
      await confirmDialog.accept();
    });
    const deleteResponsePromise = page.waitForResponse(
      isAssetDeleteResponse(createdAsset.id),
    );
    await updatedRow.getByRole("button", { name: /delete/i }).click();
    const deleteResponse = await deleteResponsePromise;
    expect(deleteResponse.status()).toBe(204);
    await expect(updatedRow).toBeHidden();
  });
});
