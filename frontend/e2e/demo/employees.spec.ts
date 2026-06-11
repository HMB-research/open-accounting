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

interface EmployeeResponse {
  id: string;
  employee_number?: string;
  first_name: string;
  last_name: string;
  email?: string;
  position?: string;
  department?: string;
  employment_type: "FULL_TIME" | "PART_TIME" | "CONTRACT";
  apply_basic_exemption: boolean;
  basic_exemption_amount: string | number;
  funded_pension_rate: string | number;
  is_active: boolean;
}

function responsePath(responseUrl: string): string {
  return new URL(responseUrl).pathname;
}

function employeesResponse(response: Response): boolean {
  const url = new URL(response.url());

  return (
    response.request().method() === "GET" &&
    response.status() === 200 &&
    responsePath(response.url()).endsWith("/employees") &&
    url.searchParams.get("active_only") === "true"
  );
}

function createEmployeeResponse(response: Response): boolean {
  return (
    response.request().method() === "POST" &&
    response.status() === 201 &&
    responsePath(response.url()).endsWith("/employees")
  );
}

function displayName(
  employee: Pick<EmployeeResponse, "first_name" | "last_name">,
): string {
  return `${employee.last_name}, ${employee.first_name}`;
}

function formatAmount(value: string | number): string {
  return Number(value).toFixed(2);
}

function formatPercent(value: string | number): string {
  return `${(Number(value) * 100).toFixed(0)}%`;
}

async function openEmployeesPage(
  page: Page,
  testInfo: TestInfo,
): Promise<EmployeeResponse[]> {
  const employeesLoaded = page.waitForResponse(employeesResponse);

  await navigateTo(page, "/employees", testInfo, {
    waitForNetworkIdle: false,
  });
  const employeesResult = await employeesLoaded;
  const employees = (await employeesResult.json()) as EmployeeResponse[];
  await waitForRouteReady(page, "main h1, table tbody tr, .empty-state");

  return employees;
}

test.describe("Demo Employees", () => {
  test.beforeEach(async ({ page }, testInfo) => {
    await ensureAuthenticated(page, testInfo);
    await ensureDemoTenant(page, testInfo);
  });

  test("verifies seeded employees and creates a test employee", async ({
    page,
  }, testInfo) => {
    const employees = await openEmployeesPage(page, testInfo);
    const rows = page.locator("table tbody tr");

    expect(employees.length).toBeGreaterThanOrEqual(4);
    expect(employees.every((employee) => employee.is_active)).toBe(true);

    await expect(
      page.getByRole("heading", { name: /employees|töötajad/i, level: 1 }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", {
        name: /import employees|impordi töötajad/i,
      }),
    ).toBeVisible();
    await expect(
      page.getByRole("button", { name: /new employee|uus töötaja/i }),
    ).toBeVisible();
    await expect(
      page.getByLabel(/active only|ainult aktiivsed/i),
    ).toBeChecked();
    await expect(
      page.getByPlaceholder(/search employees|otsi töötajaid/i),
    ).toBeVisible();

    await expect(rows).toHaveCount(employees.length);

    const maria = employees.find(
      (employee) =>
        employee.first_name === "Maria" && employee.last_name === "Tamm",
    );
    const jaan = employees.find(
      (employee) =>
        employee.first_name === "Jaan" && employee.last_name === "Kask",
    );
    expect(maria).toBeTruthy();
    expect(jaan).toBeTruthy();

    const mariaRow = rows.filter({ hasText: displayName(maria!) }).first();
    await expect(mariaRow).toContainText(maria!.employee_number || "");
    await expect(mariaRow).toContainText(maria!.email || "");
    await expect(mariaRow).toContainText(maria!.position || "");
    await expect(mariaRow).toContainText(maria!.department || "");
    await expect(mariaRow).toContainText(
      formatAmount(maria!.basic_exemption_amount),
    );
    await expect(mariaRow).toContainText(
      formatPercent(maria!.funded_pension_rate),
    );

    const jaanRow = rows.filter({ hasText: displayName(jaan!) }).first();
    await expect(jaanRow).toContainText(jaan!.position || "");
    await expect(jaanRow).toContainText(jaan!.department || "");
    await expect(page.getByText("Kivi, Liisa")).toHaveCount(0);

    const searchInput = page.getByPlaceholder(
      /search employees|otsi töötajaid/i,
    );
    await searchInput.fill("Kask");
    await expect(rows).toHaveCount(1);
    await expect(rows.first()).toContainText(displayName(jaan!));
    await searchInput.fill("");
    await expect(rows).toHaveCount(employees.length);

    await page
      .getByRole("button", {
        name: /import employees|impordi töötajad/i,
      })
      .click();
    const importDialog = page.getByRole("dialog", {
      name: /import employees|impordi töötajad/i,
    });
    await expect(importDialog).toBeVisible();
    await expect(importDialog.locator("#employee-import-file")).toBeVisible();
    await expect(
      importDialog.getByRole("button", {
        name: /download template|laadi mall alla/i,
      }),
    ).toBeVisible();
    await importDialog.getByRole("button", { name: /cancel|tühista/i }).click();
    await expect(importDialog).toBeHidden();

    const suffix = `${testInfo.workerIndex}-${testInfo.repeatEachIndex}-${Date.now()}`;
    const createdFirstName = "E2E";
    const createdLastName = `Worker${suffix.replace(/\D/g, "").slice(-8)}`;
    const createdEmail = `e2e.employee.${suffix}@example.com`;

    await page
      .getByRole("button", { name: /new employee|uus töötaja/i })
      .click();
    const createDialog = page.getByRole("dialog", {
      name: /add new employee|lisa uus töötaja/i,
    });
    await expect(createDialog).toBeVisible();
    await createDialog.locator("#firstName").fill(createdFirstName);
    await createDialog.locator("#lastName").fill(createdLastName);
    await createDialog.locator("#email").fill(createdEmail);
    await createDialog.locator("#position").fill("E2E Payroll Tester");
    await createDialog.locator("#department").fill("Quality");
    await createDialog.locator("#employmentType").selectOption("PART_TIME");
    await createDialog.locator("#pensionRate").selectOption("0.04");

    const [createdResponse] = await Promise.all([
      page.waitForResponse(createEmployeeResponse),
      createDialog
        .getByRole("button", {
          name: /^(add employee|lisa töötaja)$/i,
        })
        .click(),
    ]);
    const createdEmployee = (await createdResponse.json()) as EmployeeResponse;
    expect(createdEmployee.first_name).toBe(createdFirstName);
    expect(createdEmployee.last_name).toBe(createdLastName);
    expect(createdEmployee.email).toBe(createdEmail);
    expect(createdEmployee.position).toBe("E2E Payroll Tester");
    expect(createdEmployee.department).toBe("Quality");
    expect(createdEmployee.employment_type).toBe("PART_TIME");
    expect(createdEmployee.is_active).toBe(true);

    const createdRow = rows
      .filter({ hasText: displayName(createdEmployee) })
      .first();
    await expect(createdRow).toBeVisible();
    await expect(rows).toHaveCount(employees.length + 1);
    await expect(createdRow).toContainText(createdEmail);
    await expect(createdRow).toContainText("E2E Payroll Tester");
    await expect(createdRow).toContainText("Quality");
    await expect(createdRow).toContainText(/part-time|osaline koormus/i);
    await expect(createdRow).toContainText(
      formatAmount(createdEmployee.basic_exemption_amount),
    );
    await expect(createdRow).toContainText(
      formatPercent(createdEmployee.funded_pension_rate),
    );
  });
});
