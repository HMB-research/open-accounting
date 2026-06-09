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

// Authenticate all 4 demo users and save their auth state
// This runs once before any test workers start
setup("authenticate all demo users", async ({ browser }) => {
  // Ensure auth directory exists
  if (!fs.existsSync(AUTH_DIR)) {
    fs.mkdirSync(AUTH_DIR, { recursive: true });
  }

  console.log(
    `[Auth Setup] Authenticating all ${DEMO_CREDENTIALS.length} demo users...`,
  );

  await Promise.all(
    DEMO_CREDENTIALS.map(async (creds, workerIndex) => {
      const authFile = path.join(AUTH_DIR, `worker-${workerIndex}.json`);

      console.log(
        `[Auth Setup] Authenticating demo user ${workerIndex + 1}/${DEMO_CREDENTIALS.length}: ${creds.email}...`,
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
    `[Auth Setup] All ${DEMO_CREDENTIALS.length} demo users authenticated successfully`,
  );
});
