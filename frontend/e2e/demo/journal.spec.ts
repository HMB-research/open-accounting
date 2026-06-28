import {
  test,
  expect,
  type Locator,
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

interface AccountResponse {
  id: string;
  code: string;
  name: string;
}

interface JournalEntryLineResponse {
  account_id: string;
  description?: string;
  debit_amount: string | number;
  credit_amount: string | number;
}

interface JournalEntryResponse {
  id: string;
  entry_number: string;
  entry_date: string;
  description: string;
  reference?: string;
  status: "DRAFT" | "POSTED" | "VOIDED";
  void_reason?: string;
  lines: JournalEntryLineResponse[];
}

interface PostJournalEntryResponse {
  status: string;
}

function responsePath(responseUrl: string): string {
  return new URL(responseUrl).pathname;
}

function accountsResponse(response: Response): boolean {
  const url = new URL(response.url());

  return (
    response.request().method() === "GET" &&
    response.status() === 200 &&
    responsePath(response.url()).endsWith("/accounts") &&
    url.searchParams.get("active_only") === "true"
  );
}

function journalEntriesResponse(response: Response): boolean {
  const url = new URL(response.url());

  return (
    response.request().method() === "GET" &&
    response.status() === 200 &&
    responsePath(response.url()).endsWith("/journal-entries") &&
    url.searchParams.get("limit") === "50"
  );
}

function createJournalEntryResponse(response: Response): boolean {
  return (
    response.request().method() === "POST" &&
    response.status() === 201 &&
    responsePath(response.url()).endsWith("/journal-entries")
  );
}

function journalEntryActionResponse(
  response: Response,
  entryId: string,
  action: "post" | "void",
): boolean {
  return (
    response.request().method() === "POST" &&
    response.status() === 200 &&
    responsePath(response.url()).endsWith(
      `/journal-entries/${entryId}/${action}`,
    )
  );
}

function formatAmount(value: string | number): string {
  return Number(value).toFixed(2);
}

function numericAmount(value: string | number): number {
  return Number(value);
}

async function selectAccountByText(select: Locator, text: string) {
  const value = await select.evaluate((element, optionText) => {
    const selectElement = element as HTMLSelectElement;
    const option = Array.from(selectElement.options).find((candidate) =>
      candidate.textContent?.includes(optionText),
    );
    return option?.value || "";
  }, text);
  expect(value, `account option containing "${text}"`).not.toBe("");
  await select.selectOption(value);
}

async function openJournalPage(
  page: Page,
  testInfo: TestInfo,
): Promise<{
  accounts: AccountResponse[];
  entries: JournalEntryResponse[];
}> {
  const accountsLoaded = page.waitForResponse(accountsResponse);
  const entriesLoaded = page.waitForResponse(journalEntriesResponse);

  await navigateTo(page, "/journal", testInfo, { waitForNetworkIdle: false });
  const [accountsResult, entriesResult] = await Promise.all([
    accountsLoaded,
    entriesLoaded,
  ]);
  const [accounts, entries] = (await Promise.all([
    accountsResult.json(),
    entriesResult.json(),
  ])) as [AccountResponse[], JournalEntryResponse[]];

  await waitForRouteReady(page, "main h1, .entry-card, .empty-state");

  return { accounts, entries };
}

test.describe("Demo Journal Entries", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
  });

  test("verifies seeded ledger data and the manual entry lifecycle", async ({
    page,
  }, testInfo) => {
    const { accounts, entries } = await openJournalPage(page, testInfo);
    const description = `E2E journal lifecycle ${Date.now()}`;
    const reference = `E2E-${Date.now()}`;
    const postReason = "E2E reviewed manual entry";
    const voidReason = "E2E void after lifecycle verification";

    expect(accounts.length).toBeGreaterThan(0);
    expect(entries.length).toBeGreaterThan(0);

    await expect(
      page.getByRole("heading", {
        name: /journal entries|pearaamat/i,
        level: 1,
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /new entry|uus kanne/i }).first(),
    ).toBeVisible();
    await expect(
      page
        .getByRole("button", {
          name: /import opening balances|impordi algsaldod/i,
        })
        .first(),
    ).toBeVisible();

    const entryCards = page.locator(".entry-card");
    await expect(entryCards).toHaveCount(entries.length);

    const firstSeedEntry = entries[0];
    const firstSeedCard = entryCards
      .filter({ hasText: firstSeedEntry.entry_number })
      .first();
    await expect(firstSeedCard).toBeVisible();
    await expect(firstSeedCard).toContainText(firstSeedEntry.description);
    await expect(firstSeedCard).toContainText(firstSeedEntry.status);
    if (firstSeedEntry.reference) {
      await expect(firstSeedCard).toContainText(firstSeedEntry.reference);
    }
    await expect(firstSeedCard.locator("tbody tr")).toHaveCount(
      firstSeedEntry.lines.length,
    );

    await page
      .getByRole("button", {
        name: /import opening balances|impordi algsaldod/i,
      })
      .first()
      .click();
    const importDialog = page.getByRole("dialog", {
      name: /import opening balances|impordi algsaldod/i,
    });
    await expect(importDialog).toBeVisible();
    await expect(importDialog.locator("#opening-balance-file")).toBeVisible();
    await expect(
      importDialog.getByRole("button", {
        name: /download template|lae mall alla/i,
      }),
    ).toBeVisible();
    await importDialog.getByRole("button", { name: /cancel|tühista/i }).click();
    await expect(importDialog).toBeHidden();

    await page
      .getByRole("button", { name: /new entry|uus kanne/i })
      .first()
      .click();

    const dialog = page.getByRole("dialog", {
      name: /create journal entry|loo kanne/i,
    });
    await expect(dialog).toBeVisible();
    await dialog.locator("#description").fill(description);
    await dialog.locator("#reference").fill(reference);

    const lineRows = dialog.locator("tbody tr");
    await expect(lineRows).toHaveCount(2);

    const debitRow = lineRows.nth(0);
    await selectAccountByText(debitRow.locator("select"), "1000 - Cash");
    await debitRow.locator('input[type="text"]').fill("Lifecycle debit");
    await debitRow.locator('input[type="number"]').first().fill("125.50");

    const creditRow = lineRows.nth(1);
    await selectAccountByText(
      creditRow.locator("select"),
      "4000 - Sales Revenue",
    );
    await creditRow.locator('input[type="text"]').fill("Lifecycle credit");
    await creditRow.locator('input[type="number"]').nth(1).fill("125.50");

    await expect(dialog.getByText(/balanced|tasakaalus/i)).toBeVisible();

    const [createdResponse] = await Promise.all([
      page.waitForResponse(createJournalEntryResponse),
      dialog
        .getByRole("button", { name: /^(create entry|loo kanne)$/i })
        .click(),
    ]);
    const createdEntry = (await createdResponse.json()) as JournalEntryResponse;
    expect(createdEntry.description).toBe(description);
    expect(createdEntry.reference).toBe(reference);
    expect(createdEntry.status).toBe("DRAFT");
    expect(createdEntry.lines).toHaveLength(2);
    expect(createdEntry.lines[0].description).toBe("Lifecycle debit");
    expect(numericAmount(createdEntry.lines[0].debit_amount)).toBe(125.5);
    expect(createdEntry.lines[1].description).toBe("Lifecycle credit");
    expect(numericAmount(createdEntry.lines[1].credit_amount)).toBe(125.5);

    const entryCard = page
      .locator(".entry-card")
      .filter({ hasText: description })
      .first();
    await expect(entryCard).toBeVisible();
    await expect(entryCard).toContainText(reference);
    await expect(entryCard).toContainText("DRAFT");
    await expect(entryCard).toContainText("Lifecycle debit");
    await expect(entryCard).toContainText("Lifecycle credit");
    await expect(entryCard).toContainText(
      formatAmount(createdEntry.lines[0].debit_amount),
    );
    await expect(entryCard).toContainText(
      formatAmount(createdEntry.lines[1].credit_amount),
    );

    page.once("dialog", (prompt) => prompt.accept(postReason));
    const [postedResponse] = await Promise.all([
      page.waitForResponse((response) =>
        journalEntryActionResponse(response, createdEntry.id, "post"),
      ),
      entryCard.getByRole("button", { name: /^(post|konteeri)$/i }).click(),
    ]);
    const postedResult =
      (await postedResponse.json()) as PostJournalEntryResponse;
    expect(postedResult.status).toBe("posted");

    await expect(entryCard).toContainText("POSTED");

    page.once("dialog", (prompt) => prompt.accept(voidReason));
    const [voidedResponse] = await Promise.all([
      page.waitForResponse((response) =>
        journalEntryActionResponse(response, createdEntry.id, "void"),
      ),
      entryCard.getByRole("button", { name: /^(void|tühista)$/i }).click(),
    ]);
    const reversalEntry = (await voidedResponse.json()) as JournalEntryResponse;
    expect(reversalEntry.status).toBe("POSTED");
    expect(reversalEntry.reference).toBe(createdEntry.entry_number);
    expect(reversalEntry.lines).toHaveLength(createdEntry.lines.length);

    await expect(entryCard).toContainText("VOIDED");
    await expect(entryCard).toContainText(voidReason);
  });
});
