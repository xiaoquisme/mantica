import { existsSync } from "node:fs";
import path from "node:path";
import { defineConfig } from "@playwright/test";

// Load the same env file the backend was started with so e2e/fixtures.ts'
// direct `verification_code` query reads the SAME database the server writes
// to. `.env.worktree` wins over `.env` when both exist (matches make
// start-worktree precedence). Vars already set in process.env (e.g. via
// `DATABASE_URL=... pnpm exec playwright test`) take precedence over the
// file because `process.loadEnvFile` does not overwrite existing values.
for (const envFile of [".env.worktree", ".env"]) {
  const fullPath = path.resolve(envFile);
  if (existsSync(fullPath)) {
    process.loadEnvFile(fullPath);
  }
}

export default defineConfig({
  testDir: "./e2e",
  timeout: 30000,
  retries: 0,
  // globalSetup performs ONE OTP login and writes the JWT to disk; every
  // worker hydrates its sessionCache from that file at startup. Without
  // this, the first test in every spec file would re-run send-code, but
  // the backend rate-limits to 1 code / 10s / email AND marks codes used,
  // so the second worker (or second file) would fail "No verification
  // code found".
  globalSetup: "./e2e/global-setup.ts",
  // Run serially. All tests share one E2E workspace ("E2E Workspace"),
  // and settings.spec.ts mutates its name — running in parallel produces
  // race conditions where another worker reads/writes the workspace mid-
  // rename. The suite is small (~60s serial), so the loss of parallelism
  // is acceptable in exchange for stability against shared-state races.
  workers: 1,
  // Failure screenshots land at test-results/<test>/test-failed-*.png; pass
  // screenshots are written by the afterEach hook in e2e/fixtures.ts.
  outputDir: "test-results",
  use: {
    baseURL: process.env.PLAYWRIGHT_BASE_URL ?? process.env.FRONTEND_ORIGIN ?? "http://localhost:3000",
    headless: true,
    screenshot: "only-on-failure",
  },
  projects: [
    {
      name: "chromium",
      use: { browserName: "chromium" },
    },
  ],
  // Don't auto-start servers — they must be running already
  // This avoids complexity and port conflicts during testing
});
