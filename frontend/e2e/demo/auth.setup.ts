import { test as setup } from "@playwright/test";
import * as fs from "fs";
import * as path from "path";
import { fileURLToPath } from "url";
import { DEMO_CREDENTIALS, loginWithDemoCredentials } from "./utils";

// ESM-compatible __dirname
const __filename = fileURLToPath(import.meta.url);
const __dirname = path.dirname(__filename);

// Auth state directory - must match utils.ts and playwright.demo.config.ts
const AUTH_DIR = path.join(__dirname, "..", "..", ".auth");

// Authenticate the demo users needed by the current worker count and save their auth state.
// This runs once before any test workers start
setup("authenticate demo users", async ({ browser }, testInfo) => {
  // Ensure auth directory exists
  if (!fs.existsSync(AUTH_DIR)) {
    fs.mkdirSync(AUTH_DIR, { recursive: true });
  }

  const authWorkerCount = Math.min(
    Math.max(testInfo.config.workers, 1),
    DEMO_CREDENTIALS.length,
  );
  const authCredentials = DEMO_CREDENTIALS.slice(0, authWorkerCount);

  console.log(
    `[Auth Setup] Authenticating ${authCredentials.length}/${DEMO_CREDENTIALS.length} demo users for ${testInfo.config.workers} worker(s)...`,
  );

  await Promise.all(
    authCredentials.map(async (creds, workerIndex) => {
      const authFile = path.join(AUTH_DIR, `worker-${workerIndex}.json`);

      console.log(
        `[Auth Setup] Authenticating demo user ${workerIndex + 1}/${authCredentials.length}: ${creds.email}...`,
      );

      // Create a new context for each user
      const context = await browser.newContext();
      const page = await context.newPage();

      try {
        // Remember Me stores tokens in localStorage, which Playwright storageState preserves.
        await loginWithDemoCredentials(page, creds, {
          rememberMe: true,
          logPrefix: "[Auth Setup] Login",
        });

        console.log(
          `[Auth Setup] Login successful for ${creds.email}, saving state to ${authFile}`,
        );

        // Save authentication state
        await context.storageState({ path: authFile });
      } catch (error) {
        console.error(
          `[Auth Setup] Failed to authenticate ${creds.email}:`,
          error,
        );
        throw error;
      } finally {
        await context.close();
      }
    }),
  );

  console.log(
    `[Auth Setup] ${authCredentials.length} demo user auth state(s) saved successfully`,
  );
});
