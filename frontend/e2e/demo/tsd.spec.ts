import { test, expect, type Page, type TestInfo } from "@playwright/test";
import {
  DEMO_API_URL,
  ensureAuthenticated,
  navigateTo,
  waitForRouteReady,
} from "./utils";

const demoTsdYear = 2024;
const routeLoadTimeout = 30_000;
const apiResponseTimeout = 30_000;

interface TSDDeclarationResponse {
  id: string;
  period_year: number;
  period_month: number;
  total_payments: string | number;
  total_income_tax: string | number;
  total_social_tax: string | number;
  total_unemployment_employer: string | number;
  total_unemployment_employee: string | number;
  total_funded_pension: string | number;
  status: "DRAFT" | "SUBMITTED" | "ACCEPTED" | "REJECTED";
  emta_reference?: string | null;
}

interface TSDSelection {
  tenantId: string;
  declarations: TSDDeclarationResponse[];
}

interface DocumentResponse {
  id: string;
  file_name: string;
  document_type: "supporting_document" | "tax_support";
  review_status: "PENDING" | "REVIEWED" | "APPROVED" | "REJECTED";
}

function responsePath(responseUrl: string): string {
  return new URL(responseUrl).pathname;
}

function tenantIdFromResponse(responseUrl: string): string {
  const match = responsePath(responseUrl).match(
    /\/api\/v1\/tenants\/([^/]+)\//,
  );
  if (!match) {
    throw new Error(
      `Unable to infer tenant ID from TSD response URL: ${responseUrl}`,
    );
  }
  return match[1];
}

function formatAmount(amount: string | number): string {
  return Number(amount).toFixed(2);
}

function totalTaxes(declaration: TSDDeclarationResponse): string {
  return [
    declaration.total_income_tax,
    declaration.total_social_tax,
    declaration.total_unemployment_employer,
    declaration.total_unemployment_employee,
    declaration.total_funded_pension,
  ]
    .reduce((sum, amount) => sum + Number(amount), 0)
    .toFixed(2);
}

function statusPattern(status: TSDDeclarationResponse["status"]): RegExp {
  switch (status) {
    case "DRAFT":
      return /draft|mustand/i;
    case "SUBMITTED":
      return /submitted|esitatud/i;
    case "ACCEPTED":
      return /accepted|aktsepteeritud/i;
    case "REJECTED":
      return /rejected|tagasi/i;
  }
}

function waitForTSDListResponse(page: Page, expectedYear?: number) {
  return page.waitForResponse(
    (response) => {
      if (response.request().method() !== "GET" || response.status() !== 200) {
        return false;
      }

      const url = new URL(response.url());
      const isTsdList = /\/api\/v1\/tenants\/[^/]+\/tsd$/.test(url.pathname);
      if (!isTsdList) {
        return false;
      }

      return (
        expectedYear === undefined ||
        url.searchParams.get("year") === String(expectedYear)
      );
    },
    { timeout: apiResponseTimeout },
  );
}

function waitForTSDExportResponse(
  page: Page,
  declaration: TSDDeclarationResponse,
  format: "xml" | "csv",
) {
  return page.waitForResponse(
    (response) => {
      return (
        response.request().method() === "GET" &&
        response.status() === 200 &&
        new RegExp(
          `/api/v1/tenants/[^/]+/tsd/${declaration.period_year}/${declaration.period_month}/${format}$`,
        ).test(responsePath(response.url()))
      );
    },
    { timeout: apiResponseTimeout },
  );
}

function waitForTSDSubmitResponse(
  page: Page,
  declaration: TSDDeclarationResponse,
  expectedStatus = 200,
) {
  return page.waitForResponse(
    (response) => {
      return (
        response.request().method() === "POST" &&
        response.status() === expectedStatus &&
        new RegExp(
          `/api/v1/tenants/[^/]+/tsd/${declaration.period_year}/${declaration.period_month}/submit$`,
        ).test(responsePath(response.url()))
      );
    },
    { timeout: apiResponseTimeout },
  );
}

async function demoApiRequest<T>(
  page: Page,
  path: string,
  options: {
    method?: string;
    body?: unknown;
    form?: Record<string, string>;
    file?: { field: string; name: string; content: string; type: string };
  } = {},
): Promise<{ status: number; body: T }> {
  return page.evaluate(
    async ({ apiUrl, requestPath, requestOptions }) => {
      const token =
        localStorage.getItem("access_token") ||
        sessionStorage.getItem("access_token");
      if (!token) {
        throw new Error("Missing demo access token");
      }

      const headers: Record<string, string> = {
        Authorization: `Bearer ${token}`,
      };
      let body: BodyInit | undefined;

      if (requestOptions.form || requestOptions.file) {
        const formData = new FormData();
        for (const [key, value] of Object.entries(requestOptions.form || {})) {
          formData.set(key, value);
        }
        if (requestOptions.file) {
          formData.set(
            requestOptions.file.field,
            new File([requestOptions.file.content], requestOptions.file.name, {
              type: requestOptions.file.type,
            }),
          );
        }
        body = formData;
      } else if (requestOptions.body !== undefined) {
        headers["Content-Type"] = "application/json";
        body = JSON.stringify(requestOptions.body);
      }

      const response = await fetch(`${apiUrl}${requestPath}`, {
        method: requestOptions.method || "GET",
        headers,
        body,
      });
      const text = await response.text();
      let parsed: unknown = {};
      if (text) {
        try {
          parsed = JSON.parse(text);
        } catch {
          parsed = { raw: text };
        }
      }
      return { status: response.status, body: parsed };
    },
    { apiUrl: DEMO_API_URL, requestPath: path, requestOptions: options },
  ) as Promise<{ status: number; body: T }>;
}

async function attachApprovedTSDEvidence(
  page: Page,
  tenantId: string,
  declaration: TSDDeclarationResponse,
  suffix: string,
): Promise<DocumentResponse> {
  const fileName = `tsd-${declaration.period_year}-${String(declaration.period_month).padStart(2, "0")}-${suffix}.txt`;
  const upload = await demoApiRequest<DocumentResponse>(
    page,
    `/api/v1/tenants/${tenantId}/documents`,
    {
      method: "POST",
      form: {
        entity_type: "tsd_declaration",
        entity_id: declaration.id,
        document_type: "tax_support",
        notes: `E2E e-MTA submission evidence ${suffix}`,
      },
      file: {
        field: "file",
        name: fileName,
        content: `Approved e-MTA submission evidence for ${declaration.period_year}-${declaration.period_month}`,
        type: "text/plain",
      },
    },
  );
  expect(upload.status).toBe(201);
  expect(upload.body.review_status).toBe("PENDING");

  const review = await demoApiRequest<DocumentResponse>(
    page,
    `/api/v1/tenants/${tenantId}/documents/${upload.body.id}/review`,
    {
      method: "POST",
      body: {
        review_status: "APPROVED",
        review_note: "Approved by TSD demo E2E",
      },
    },
  );
  expect(review.status).toBe(200);
  expect(review.body.review_status).toBe("APPROVED");
  expect(review.body.document_type).toBe("tax_support");
  return review.body;
}

async function hasApprovedTSDSubmissionEvidence(
  page: Page,
  tenantId: string,
  declaration: TSDDeclarationResponse,
): Promise<boolean> {
  const query = new URLSearchParams({
    entity_type: "tsd_declaration",
    entity_id: declaration.id,
  });
  const response = await demoApiRequest<DocumentResponse[]>(
    page,
    `/api/v1/tenants/${tenantId}/documents?${query.toString()}`,
  );
  expect(response.status).toBe(200);
  return response.body.some(
    (document) =>
      document.review_status === "APPROVED" &&
      (document.document_type === "tax_support" ||
        document.document_type === "supporting_document"),
  );
}

async function waitForTSDLoaded(page: Page): Promise<void> {
  await waitForRouteReady(
    page,
    "table tbody tr, .empty-state",
    routeLoadTimeout,
  );
  await expect(
    page.getByText(/^Loading\.\.\.$|^Laadimine\.\.\.$/i),
  ).toHaveCount(0, {
    timeout: routeLoadTimeout,
  });
}

async function openTSD(page: Page, testInfo: TestInfo): Promise<void> {
  const initialYear = new Date().getFullYear();
  const listPromise = waitForTSDListResponse(page, initialYear);
  await navigateTo(page, "/tsd", testInfo, { waitForNetworkIdle: false });
  expect((await listPromise).ok()).toBeTruthy();
  await waitForRouteReady(page, "main h1, .container h1", routeLoadTimeout);
  await waitForTSDLoaded(page);
}

async function selectTSDYear(page: Page, year: number): Promise<TSDSelection> {
  const listPromise = waitForTSDListResponse(page, year);
  await page.locator("select#yearFilter").selectOption(String(year));
  const response = await listPromise;
  expect(response.ok()).toBeTruthy();
  await waitForTSDLoaded(page);
  return {
    tenantId: tenantIdFromResponse(response.url()),
    declarations: (await response.json()) as TSDDeclarationResponse[],
  };
}

test.describe("Demo TSD Declarations", () => {
  test("covers list, exports, and manual submission controls", async ({
    page,
  }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await openTSD(page, testInfo);

    await expect(
      page.getByRole("heading", { level: 1, name: /tsd/i }),
    ).toBeVisible();
    await expect(page.locator(".info-banner")).toContainText(
      /e-mta|emta|manual|käsitsi/i,
    );
    await expect(
      page.locator('.info-banner a[href="https://www.emta.ee"]'),
    ).toBeVisible();

    const yearFilter = page.locator("select#yearFilter");
    await expect(yearFilter).toBeVisible();
    const yearOptions = await yearFilter.locator("option").allTextContents();
    expect(yearOptions).toContain(String(demoTsdYear));

    const { tenantId, declarations } = await selectTSDYear(page, demoTsdYear);
    expect(declarations.length).toBeGreaterThanOrEqual(3);
    expect(
      declarations.map(
        (declaration) =>
          `${declaration.period_year}-${declaration.period_month}`,
      ),
    ).toEqual(expect.arrayContaining(["2024-10", "2024-11", "2024-12"]));
    expect(
      declarations.some((declaration) => declaration.status === "DRAFT"),
    ).toBeTruthy();
    expect(
      declarations.some((declaration) => declaration.status === "SUBMITTED"),
    ).toBeTruthy();

    const rows = page.locator("table tbody tr");
    await expect(rows).toHaveCount(declarations.length);

    const firstDeclaration = declarations[0];
    const firstRow = rows.first();
    await expect(firstRow).toContainText(String(firstDeclaration.period_year));
    await expect(firstRow).toContainText(
      formatAmount(firstDeclaration.total_payments),
    );
    await expect(firstRow).toContainText(
      formatAmount(firstDeclaration.total_income_tax),
    );
    await expect(firstRow).toContainText(
      formatAmount(firstDeclaration.total_social_tax),
    );
    await expect(firstRow).toContainText(totalTaxes(firstDeclaration));
    await expect(firstRow).toContainText(
      statusPattern(firstDeclaration.status),
    );
    await expect(firstRow.getByRole("button", { name: "XML" })).toBeVisible();
    await expect(firstRow.getByRole("button", { name: "CSV" })).toBeVisible();

    const xmlResponsePromise = waitForTSDExportResponse(
      page,
      firstDeclaration,
      "xml",
    );
    await firstRow.getByRole("button", { name: "XML" }).click();
    expect((await xmlResponsePromise).ok()).toBeTruthy();

    const csvResponsePromise = waitForTSDExportResponse(
      page,
      firstDeclaration,
      "csv",
    );
    await firstRow.getByRole("button", { name: "CSV" }).click();
    expect((await csvResponsePromise).ok()).toBeTruthy();

    const draftIndex = declarations.findIndex(
      (declaration) => declaration.status === "DRAFT",
    );
    expect(draftIndex).toBeGreaterThanOrEqual(0);
    const draftRow = rows.nth(draftIndex);
    await draftRow
      .getByRole("button", { name: /mark.*submitted|märgi.*esitatuks/i })
      .click();

    const submitDialog = page.getByRole("dialog");
    await expect(submitDialog).toBeVisible();
    await expect(
      submitDialog.getByRole("heading", { name: /mark.*tsd|märgi.*tsd/i }),
    ).toBeVisible();
    const referenceInput = submitDialog.getByLabel(
      /emta|e-mta|reference|viite/i,
    );
    await expect(referenceInput).toBeVisible();
    await submitDialog.getByRole("button", { name: /cancel|tühista/i }).click();
    await expect(submitDialog).toBeHidden();

    await draftRow
      .getByRole("button", { name: /mark.*submitted|märgi.*esitatuks/i })
      .click();
    await expect(submitDialog).toBeVisible();
    const emtaReference = `EMTA-E2E-${Date.now()}`;
    await referenceInput.fill(emtaReference);

    if (
      !(await hasApprovedTSDSubmissionEvidence(
        page,
        tenantId,
        declarations[draftIndex],
      ))
    ) {
      const blockedSubmitResponsePromise = waitForTSDSubmitResponse(
        page,
        declarations[draftIndex],
        409,
      );
      await submitDialog
        .getByRole("button", { name: /mark.*submitted|märgi.*esitatuks/i })
        .click();
      const blockedSubmitResponse = await blockedSubmitResponsePromise;
      expect(blockedSubmitResponse.ok()).toBeFalsy();
      const blockedSubmitBody = await blockedSubmitResponse.json();
      expect(blockedSubmitBody.error).toContain(
        "approved TSD submission evidence is required",
      );
      await expect(submitDialog).toBeVisible();

      await attachApprovedTSDEvidence(
        page,
        tenantId,
        declarations[draftIndex],
        emtaReference,
      );
    }

    const submitResponsePromise = waitForTSDSubmitResponse(
      page,
      declarations[draftIndex],
    );
    const refreshResponsePromise = waitForTSDListResponse(page, demoTsdYear);
    await submitDialog
      .getByRole("button", { name: /mark.*submitted|märgi.*esitatuks/i })
      .click();

    const submitResponse = await submitResponsePromise;
    expect(submitResponse.ok()).toBeTruthy();
    expect(submitResponse.request().postDataJSON()).toMatchObject({
      emta_reference: emtaReference,
    });

    const refreshedDeclarations = (await (
      await refreshResponsePromise
    ).json()) as TSDDeclarationResponse[];
    const submittedDeclaration = refreshedDeclarations.find(
      (declaration) =>
        declaration.period_year === declarations[draftIndex].period_year &&
        declaration.period_month === declarations[draftIndex].period_month,
    );
    expect(submittedDeclaration?.status).toBe("SUBMITTED");
    expect(submittedDeclaration?.emta_reference).toBe(emtaReference);

    await expect(submitDialog).toBeHidden();
    await waitForTSDLoaded(page);
    await expect(rows.nth(draftIndex)).toContainText(
      statusPattern("SUBMITTED"),
    );
    await expect(rows.nth(draftIndex)).toContainText(emtaReference);
    await expect(
      rows
        .nth(draftIndex)
        .getByRole("button", { name: /mark.*submitted|märgi.*esitatuks/i }),
    ).toHaveCount(0);

    await expect(page.locator(".workflow-steps li")).toHaveCount(6);
    await expect(page.locator(".workflow-info")).toContainText(
      /xml|e-mta|emta|tsd/i,
    );
  });
});
