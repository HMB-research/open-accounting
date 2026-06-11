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
  asset_number: string;
  name: string;
  status: string;
}

function isAssetsListResponse(response: Response): boolean {
  return (
    response.request().method() === "GET" &&
    response.status() === 200 &&
    /\/api\/v1\/tenants\/[^/]+\/assets$/.test(new URL(response.url()).pathname)
  );
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

  test("renders seeded asset controls and table details", async ({
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
  });

  test("filters assets by status", async ({ page }, testInfo) => {
    await openAssets(page, testInfo);

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
});
